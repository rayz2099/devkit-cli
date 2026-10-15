package dsn

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"github.com/twmb/franz-go/pkg/sasl/plain"
)

type kafkaRec struct {
	Partition int
	Offset    string
	Timestamp int64
	Key       any
	Value     any
	Headers   map[string]any
}

const (
	partMaxBytes  = 262144
	fetchMaxBytes = 1048576
)

func queryKafka(ctx context.Context, raw, stmt string, connect, exec time.Duration) (QueryOut, error) {
	cl, err := openKafka(raw, connect, exec)
	if err != nil {
		return QueryOut{}, err
	}
	defer cl.Close()
	if strings.TrimSpace(stmt) == "ping" {
		if _, err := kafkaMeta(ctx, cl, nil); err != nil {
			return QueryOut{}, exitf(3, "%s", err.Error())
		}
		return QueryOut{Columns: []string{"ok"}, Rows: []map[string]any{{"ok": 1}}}, nil
	}
	parsed, err := parseKafkaStmt(stmt)
	if err != nil {
		return QueryOut{}, err
	}
	switch parsed.Head {
	case "topics":
		return listKafkaTopics(ctx, cl)
	case "peek":
		recs, err := peekKafka(ctx, cl, parsed, exec)
		if err != nil {
			return QueryOut{}, err
		}
		return recsOut(recs), nil
	default:
		recs, err := listenKafka(ctx, cl, parsed, exec)
		if err != nil {
			return QueryOut{}, err
		}
		return recsOut(recs), nil
	}
}

func openKafka(raw string, connect, exec time.Duration) (*kgo.Client, error) {
	spec, err := kafkaSpec(raw)
	if err != nil {
		return nil, err
	}
	opts := []kgo.Opt{
		kgo.SeedBrokers(spec.broker),
		kgo.ClientID("dsn-cli"),
		kgo.DialTimeout(connect),
		kgo.RequestRetries(0),
		kgo.RetryTimeout(exec),
		kgo.MetadataMaxAge(5 * time.Minute),
	}
	if spec.tls {
		opts = append(opts, kgo.DialTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	}
	if spec.user != "" || spec.pass != "" {
		opts = append(opts, kgo.SASL(plain.Auth{User: spec.user, Pass: spec.pass}.AsMechanism()))
	}
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, exitf(3, "%s", err.Error())
	}
	return cl, nil
}

type kafkaDial struct {
	broker string
	tls    bool
	user   string
	pass   string
}

func kafkaSpec(raw string) (kafkaDial, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return kafkaDial{}, exitf(1, "invalid url")
	}
	host := parsed.Hostname()
	if host == "" {
		return kafkaDial{}, exitf(1, "kafka url host is required")
	}
	port := parsed.Port()
	if port == "" {
		port = "9092"
	}
	pass, _ := parsed.User.Password()
	return kafkaDial{
		broker: host + ":" + port,
		tls:    parsed.Scheme == "kafkas",
		user:   parsed.User.Username(),
		pass:   pass,
	}, nil
}

func kafkaMeta(ctx context.Context, cl *kgo.Client, topic *string) (*kmsg.MetadataResponse, error) {
	req := kmsg.NewPtrMetadataRequest()
	req.AllowAutoTopicCreation = false
	if topic != nil {
		name := *topic
		req.Topics = []kmsg.MetadataRequestTopic{{Topic: &name}}
	}
	resp, err := cl.Request(ctx, req)
	if err != nil {
		return nil, err
	}
	meta, ok := resp.(*kmsg.MetadataResponse)
	if !ok {
		return nil, exitf(3, "unexpected metadata response")
	}
	return meta, nil
}

func listKafkaTopics(ctx context.Context, cl *kgo.Client) (QueryOut, error) {
	meta, err := kafkaMeta(ctx, cl, nil)
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	var rows []map[string]any
	for _, item := range meta.Topics {
		name := ""
		if item.Topic != nil {
			name = *item.Topic
		}
		if item.IsInternal || strings.HasPrefix(name, "__") {
			continue
		}
		replicas := 0
		if len(item.Partitions) > 0 {
			replicas = len(item.Partitions[0].Replicas)
		}
		rows = append(rows, map[string]any{
			"name":       name,
			"partitions": len(item.Partitions),
			"replicas":   replicas,
		})
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	return QueryOut{Columns: []string{"name", "partitions", "replicas"}, Rows: rows}, nil
}

func peekKafka(ctx context.Context, cl *kgo.Client, stmt KafkaStmt, exec time.Duration) ([]kafkaRec, error) {
	parts, err := topicParts(ctx, cl, stmt.Topic, stmt.Partition)
	if err != nil {
		return nil, err
	}
	windows, err := peekWindows(ctx, cl, stmt.Topic, parts, stmt.N)
	if err != nil {
		return nil, err
	}
	wait := exec
	if wait > time.Second {
		wait = time.Second
	}
	recs, err := fetchWindows(ctx, cl, stmt.Topic, windows, wait, stmt.N)
	if err != nil {
		return nil, err
	}
	return pickNewest(recs, stmt.N), nil
}

func listenKafka(ctx context.Context, cl *kgo.Client, stmt KafkaStmt, exec time.Duration) ([]kafkaRec, error) {
	parts, err := topicParts(ctx, cl, stmt.Topic, stmt.Partition)
	if err != nil {
		return nil, err
	}
	highs, err := offsetsOf(ctx, cl, stmt.Topic, parts, false)
	if err != nil {
		return nil, err
	}
	cursors := make([]kWindow, 0, len(parts))
	for _, part := range parts {
		high := highs[part]
		cursors = append(cursors, kWindow{Part: part, Start: high, Stop: high + 1_000_000_000})
	}
	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	deadline := time.Now().Add(exec)
	var recs []kafkaRec
	for sigCtx.Err() == nil && time.Now().Before(deadline) && len(recs) < kafkaMax {
		remain := time.Until(deadline)
		if remain <= 0 {
			break
		}
		wait := 5 * time.Second
		if remain < wait {
			wait = remain
		}
		batch, err := fetchWindows(sigCtx, cl, stmt.Topic, cursors, wait, kafkaMax-len(recs))
		if err != nil {
			if sigCtx.Err() != nil {
				break
			}
			return nil, err
		}
		recs = append(recs, batch...)
		cursors = advance(cursors, batch)
		if len(batch) == 0 {
			continue
		}
	}
	if len(recs) > kafkaMax {
		recs = recs[:kafkaMax]
	}
	return recs, nil
}

func topicParts(ctx context.Context, cl *kgo.Client, topic string, partition *int) ([]int32, error) {
	meta, err := kafkaMeta(ctx, cl, &topic)
	if err != nil {
		return nil, exitf(3, "%s", err.Error())
	}
	var found *kmsg.MetadataResponseTopic
	for i := range meta.Topics {
		item := &meta.Topics[i]
		if item.Topic != nil && *item.Topic == topic {
			found = item
			break
		}
	}
	if found == nil || found.ErrorCode != 0 || len(found.Partitions) == 0 {
		return nil, exitf(3, "unknown topic %s", topic)
	}
	if partition == nil {
		out := make([]int32, len(found.Partitions))
		for i, part := range found.Partitions {
			out[i] = part.Partition
		}
		return out, nil
	}
	want := int32(*partition)
	for _, part := range found.Partitions {
		if part.Partition == want {
			return []int32{want}, nil
		}
	}
	return nil, exitf(3, "unknown partition %d", *partition)
}

type kWindow struct {
	Part  int32
	Start int64
	Stop  int64
}

func peekWindows(ctx context.Context, cl *kgo.Client, topic string, parts []int32, n int) ([]kWindow, error) {
	highs, err := offsetsOf(ctx, cl, topic, parts, false)
	if err != nil {
		return nil, err
	}
	lows, err := offsetsOf(ctx, cl, topic, parts, true)
	if err != nil {
		return nil, err
	}
	var out []kWindow
	take := int64(n)
	for _, part := range parts {
		high := highs[part]
		low := lows[part]
		if high <= low {
			continue
		}
		start := high - take
		if start < low {
			start = low
		}
		out = append(out, kWindow{Part: part, Start: start, Stop: high})
	}
	return out, nil
}

func offsetsOf(ctx context.Context, cl *kgo.Client, topic string, parts []int32, fromBeginning bool) (map[int32]int64, error) {
	req := kmsg.NewPtrListOffsetsRequest()
	req.ReplicaID = -1
	ts := int64(-1)
	if fromBeginning {
		ts = -2
	}
	topicReq := kmsg.NewListOffsetsRequestTopic()
	topicReq.Topic = topic
	for _, part := range parts {
		p := kmsg.NewListOffsetsRequestTopicPartition()
		p.Partition = part
		p.Timestamp = ts
		topicReq.Partitions = append(topicReq.Partitions, p)
	}
	req.Topics = []kmsg.ListOffsetsRequestTopic{topicReq}
	resp, err := cl.Request(ctx, req)
	if err != nil {
		return nil, exitf(3, "%s", err.Error())
	}
	list, ok := resp.(*kmsg.ListOffsetsResponse)
	if !ok {
		return nil, exitf(3, "unexpected list offsets response")
	}
	out := map[int32]int64{}
	for _, topicRes := range list.Topics {
		for _, part := range topicRes.Partitions {
			if part.ErrorCode != 0 {
				return nil, exitf(3, "kafka partition %d error %d", part.Partition, part.ErrorCode)
			}
			out[part.Partition] = part.Offset
		}
	}
	return out, nil
}

func fetchWindows(ctx context.Context, cl *kgo.Client, topic string, windows []kWindow, wait time.Duration, remain int) ([]kafkaRec, error) {
	if len(windows) == 0 || remain <= 0 {
		return nil, nil
	}
	offsets := map[string]map[int32]kgo.Offset{topic: {}}
	for _, win := range windows {
		offsets[topic][win.Part] = kgo.NewOffset().At(win.Start)
	}
	cl.AddConsumePartitions(offsets)
	defer cl.RemoveConsumePartitions(map[string][]int32{topic: partIDs(windows)})
	fctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	fetches := cl.PollFetches(fctx)
	if err := fetches.Err(); err != nil && fctx.Err() == nil && ctx.Err() == nil {
		return nil, exitf(3, "%s", err.Error())
	}
	var recs []kafkaRec
	fetches.EachRecord(func(rec *kgo.Record) {
		if len(recs) >= remain {
			return
		}
		stop := stopOf(windows, rec.Partition)
		if rec.Offset >= stop {
			return
		}
		recs = append(recs, toRec(rec))
	})
	return recs, nil
}

func partIDs(windows []kWindow) []int32 {
	out := make([]int32, len(windows))
	for i, win := range windows {
		out[i] = win.Part
	}
	return out
}

func stopOf(windows []kWindow, part int32) int64 {
	for _, win := range windows {
		if win.Part == part {
			return win.Stop
		}
	}
	return 1 << 62
}

func toRec(rec *kgo.Record) kafkaRec {
	return kafkaRec{
		Partition: int(rec.Partition),
		Offset:    strconv.FormatInt(rec.Offset, 10),
		Timestamp: rec.Timestamp.UnixMilli(),
		Key:       decodeBytes(rec.Key),
		Value:     decodeBytes(rec.Value),
		Headers:   decodeHeaders(rec.Headers),
	}
}

func decodeHeaders(headers []kgo.RecordHeader) map[string]any {
	out := map[string]any{}
	for _, header := range headers {
		val := decodeBytes(header.Value)
		prev, ok := out[header.Key]
		if !ok {
			out[header.Key] = val
			continue
		}
		if arr, isArr := prev.([]any); isArr {
			out[header.Key] = append(arr, val)
			continue
		}
		out[header.Key] = []any{prev, val}
	}
	return out
}

func advance(cursors []kWindow, batch []kafkaRec) []kWindow {
	last := map[int32]int64{}
	for _, rec := range batch {
		off, err := strconv.ParseInt(rec.Offset, 10, 64)
		if err != nil {
			continue
		}
		last[int32(rec.Partition)] = off + 1
	}
	out := make([]kWindow, len(cursors))
	for i, cur := range cursors {
		if next, ok := last[cur.Part]; ok {
			cur.Start = next
		}
		out[i] = cur
	}
	return out
}

func recsOut(recs []kafkaRec) QueryOut {
	rows := make([]map[string]any, len(recs))
	for i, rec := range recs {
		ts := ""
		if rec.Timestamp != 0 || rec.Offset != "" {
			ts = time.UnixMilli(rec.Timestamp).UTC().Format("2006-01-02T15:04:05.000Z")
		}
		rows[i] = map[string]any{
			"partition": rec.Partition,
			"offset":    rec.Offset,
			"timestamp": ts,
			"key":       rec.Key,
			"value":     rec.Value,
			"headers":   rec.Headers,
		}
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	return QueryOut{
		Columns: []string{"partition", "offset", "timestamp", "key", "value", "headers"},
		Rows:    rows,
	}
}

// pickNewest 跨分区按时间取最后 n 条，并保持时间顺序。
func pickNewest(rows []kafkaRec, n int) []kafkaRec {
	sorted := append([]kafkaRec(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return cmpRec(sorted[i], sorted[j]) < 0
	})
	if len(sorted) <= n {
		return sorted
	}
	return sorted[len(sorted)-n:]
}

func cmpRec(a, b kafkaRec) int {
	if a.Timestamp != b.Timestamp {
		if a.Timestamp < b.Timestamp {
			return -1
		}
		return 1
	}
	if a.Partition != b.Partition {
		return a.Partition - b.Partition
	}
	ao, _ := strconv.ParseInt(a.Offset, 10, 64)
	bo, _ := strconv.ParseInt(b.Offset, 10, 64)
	if ao < bo {
		return -1
	}
	if ao > bo {
		return 1
	}
	return 0
}

// decodeBytes 把能当 JSON 的文本展开，二进制才落到 base64，避免人读结果全是编码。
func decodeBytes(buf []byte) any {
	if buf == nil {
		return nil
	}
	if len(buf) == 0 {
		return ""
	}
	if !utf8.Valid(buf) {
		return base64.StdEncoding.EncodeToString(buf)
	}
	text := string(buf)
	trim := strings.TrimLeft(text, " \t\r\n")
	if strings.HasPrefix(trim, "{") || strings.HasPrefix(trim, "[") {
		var parsed any
		if err := json.Unmarshal(buf, &parsed); err == nil {
			return parsed
		}
	}
	return text
}
