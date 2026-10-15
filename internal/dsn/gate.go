package dsn

import (
	"encoding/json"
	"strings"
	"unicode"
)

// write 原样转发；read 必须在驱动出站前按 kind 的闭集拦住。
func gateQuery(kind, access, stmt string) error {
	if access == "write" {
		return nil
	}
	switch kind {
	case "mysql", "doris", "postgres":
		return gateSQL(kind, stmt)
	case "redis":
		return gateRedis(stmt)
	case "mongodb":
		return gateMongo(stmt)
	case "elasticsearch":
		return gateES(stmt)
	case "kafka":
		_, err := parseKafkaStmt(stmt)
		return err
	default:
		return exitf(2, "blocked head: %s", kind)
	}
}

func gateSQL(kind, stmt string) error {
	dialect := "mysql"
	if kind == "postgres" {
		dialect = "postgres"
	}
	words := sqlWords(stmt, dialect)
	if len(words) == 0 {
		return exitf(2, "empty statement")
	}
	head := words[0]
	if !sqlAllow(kind, head) {
		return exitf(2, "blocked head: %s", head)
	}
	if sqlHasTail(stmt, dialect) {
		return exitf(2, "multi-statement is blocked")
	}
	if kind == "postgres" {
		if pgCatalogHead(stmt) != "" {
			_, err := parsePgCatalog(stmt)
			return err
		}
		return gatePgTails(head, words)
	}
	if head == "SELECT" || head == "WITH" {
		if tail := matchTail(words, selectTails); tail != "" {
			return exitf(2, "blocked %s", tail)
		}
	}
	return nil
}

func sqlAllow(kind, head string) bool {
	switch kind {
	case "doris":
		return head == "SELECT" || head == "WITH" || head == "SHOW" || head == "EXPLAIN" ||
			head == "DESC" || head == "DESCRIBE" || head == "USE" || head == "SWITCH"
	case "postgres":
		switch head {
		case "SELECT", "WITH", "TABLE", "VALUES", "TABLES", "COLUMNS", "DDL", "DESC", "DESCRIBE", "SHOW", "EXPLAIN":
			return true
		default:
			return false
		}
	default:
		switch head {
		case "SELECT", "WITH", "SHOW", "EXPLAIN", "DESC", "DESCRIBE", "USE":
			return true
		default:
			return false
		}
	}
}

var selectTails = [][]string{
	{"INTO", "OUTFILE"},
	{"INTO", "DUMPFILE"},
	{"FOR", "UPDATE"},
	{"FOR", "SHARE"},
	{"LOCK", "IN", "SHARE", "MODE"},
}

var pgLockTails = [][]string{
	{"FOR", "UPDATE"},
	{"FOR", "SHARE"},
	{"FOR", "NO", "KEY", "UPDATE"},
	{"FOR", "KEY", "SHARE"},
}

var pgInnerHeads = map[string]struct{}{
	"SELECT": {}, "WITH": {}, "TABLE": {}, "VALUES": {},
	"INSERT": {}, "UPDATE": {}, "DELETE": {}, "MERGE": {},
	"CREATE": {}, "DROP": {}, "ALTER": {}, "TRUNCATE": {},
	"REFRESH": {}, "CALL": {}, "DO": {}, "COPY": {},
	"DECLARE": {}, "PREPARE": {}, "EXECUTE": {}, "VACUUM": {},
}

func gatePgTails(head string, words []string) error {
	if head == "SHOW" && len(words) > 1 && words[1] == "TABLES" {
		return exitf(2, "blocked SHOW TABLES; use TABLES or TABLES LIKE <pattern>")
	}
	if head == "SHOW" && len(words) > 1 && words[1] == "CREATE" {
		return exitf(2, "blocked SHOW CREATE; use DDL <table>")
	}
	if head == "SHOW" && len(words) > 1 && words[1] == "COLUMNS" {
		return exitf(2, "blocked SHOW COLUMNS; use COLUMNS <table>")
	}
	if head == "SELECT" || head == "WITH" || head == "TABLE" || head == "VALUES" {
		if tail := matchTail(words, pgLockTails); tail != "" {
			return exitf(2, "blocked %s", tail)
		}
	}
	if (head == "SELECT" || head == "WITH") && containsSeq(words, []string{"INTO"}) {
		return exitf(2, "blocked INTO")
	}
	if head == "EXPLAIN" {
		return gateExplainAnalyze(words)
	}
	return nil
}

// EXPLAIN ANALYZE 会执行内层语句。只拦选项位上的 ANALYZE，不拦 FROM analyze。
func gateExplainAnalyze(words []string) error {
	analyzeAt := -1
	for i, word := range words {
		if word == "ANALYZE" {
			analyzeAt = i
			break
		}
	}
	if analyzeAt < 0 {
		return nil
	}
	innerAt := -1
	for i, word := range words {
		if i == 0 {
			continue
		}
		if _, ok := pgInnerHeads[word]; ok {
			innerAt = i
			break
		}
	}
	if innerAt < 0 || analyzeAt < innerAt {
		return exitf(2, "blocked ANALYZE")
	}
	return nil
}

func gateRedis(stmt string) error {
	args := splitArgs(stmt)
	if len(args) == 0 || args[0] == "" {
		return exitf(2, "empty statement")
	}
	head := strings.ToUpper(args[0])
	if !redisRead(head) {
		return exitf(2, "blocked head: %s", head)
	}
	return nil
}

func redisRead(head string) bool {
	switch head {
	case "GET", "MGET", "HGET", "HGETALL", "LRANGE", "SMEMBERS", "ZRANGE", "KEYS", "SCAN",
		"TYPE", "TTL", "EXISTS", "INFO", "PING", "LLEN", "SCARD", "ZCARD", "HLEN", "STRLEN", "DBSIZE":
		return true
	default:
		return false
	}
}

func gateMongo(stmt string) error {
	dec := json.NewDecoder(strings.NewReader(stmt))
	tok, err := dec.Token()
	if err != nil {
		return exitf(2, "invalid mongo command JSON")
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return exitf(2, "mongo command must be a JSON object")
	}
	tok, err = dec.Token()
	if err != nil {
		return exitf(2, "empty mongo command")
	}
	key, ok := tok.(string)
	if !ok {
		return exitf(2, "empty mongo command")
	}
	if !mongoRead(key) {
		return exitf(2, "blocked head: %s", key)
	}
	return nil
}

func mongoRead(key string) bool {
	switch key {
	case "find", "aggregate", "count", "distinct", "listCollections", "listIndexes", "listDatabases",
		"collStats", "dbStats", "explain", "ping", "hello", "getMore", "countDocuments", "estimatedDocumentCount":
		return true
	default:
		return false
	}
}

func gateES(stmt string) error {
	parsed, err := parseRestQuery(stmt)
	if err != nil {
		return err
	}
	if parsed.Method != "GET" && parsed.Method != "HEAD" {
		return exitf(2, "blocked head: %s", parsed.Method)
	}
	return nil
}

type restQuery struct {
	Method string
	Path   string
	Body   json.RawMessage
}

// parseRestQuery 把一行 REST 拆开。非法形状是门禁错误，不是驱动错误。
func parseRestQuery(stmt string) (restQuery, error) {
	trimmed := strings.TrimSpace(stmt)
	parts := strings.SplitN(trimmed, " ", 3)
	if len(parts) < 2 || parts[0] == "" || strings.Contains(parts[0], "\n") {
		return restQuery{}, exitf(2, "elasticsearch query must be <METHOD> /path")
	}
	method := parts[0]
	if !isAlpha(method) {
		return restQuery{}, exitf(2, "elasticsearch query must be <METHOD> /path")
	}
	path := parts[1]
	if path == "" || strings.ContainsAny(path, " \t\r\n") {
		return restQuery{}, exitf(2, "elasticsearch query must be <METHOD> /path")
	}
	method = strings.ToUpper(method)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	out := restQuery{Method: method, Path: path}
	if len(parts) < 3 {
		return out, nil
	}
	raw := strings.TrimSpace(parts[2])
	if raw == "" {
		return out, nil
	}
	if !json.Valid([]byte(raw)) {
		return restQuery{}, exitf(2, "invalid elasticsearch JSON body")
	}
	out.Body = json.RawMessage(raw)
	return out, nil
}

func isAlpha(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

type sqlTok struct {
	kind  string
	value string
}

func sqlWords(stmt, dialect string) []string {
	toks := tokenizeSQL(stmt, dialect)
	var words []string
	for _, tok := range toks {
		if tok.kind == "word" {
			words = append(words, tok.value)
		}
	}
	return words
}

func sqlHasTail(stmt, dialect string) bool {
	toks := tokenizeSQL(stmt, dialect)
	semi := -1
	for i, tok := range toks {
		if tok.kind == "semi" {
			semi = i
			break
		}
	}
	if semi < 0 {
		return false
	}
	for _, tok := range toks[semi+1:] {
		if tok.kind != "semi" {
			return true
		}
	}
	return false
}

func matchTail(words []string, tails [][]string) string {
	for _, seq := range tails {
		if containsSeq(words, seq) {
			return strings.Join(seq, " ")
		}
	}
	return ""
}

func containsSeq(words, seq []string) bool {
	if len(seq) == 0 || len(words) < len(seq) {
		return false
	}
	for i := 0; i <= len(words)-len(seq); i++ {
		ok := true
		for off := range seq {
			if words[i+off] != seq[off] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func tokenizeSQL(input, dialect string) []sqlTok {
	var toks []sqlTok
	i := 0
	for i < len(input) {
		ch := input[i]
		next := byte(0)
		if i+1 < len(input) {
			next = input[i+1]
		}
		if isSQLSpace(ch) {
			i++
			continue
		}
		if ch == '-' && next == '-' {
			i += 2
			for i < len(input) && input[i] != '\n' {
				i++
			}
			continue
		}
		if dialect == "mysql" && ch == '#' {
			i++
			for i < len(input) && input[i] != '\n' {
				i++
			}
			continue
		}
		if ch == '/' && next == '*' {
			i += 2
			for i < len(input) && !(input[i] == '*' && i+1 < len(input) && input[i+1] == '/') {
				i++
			}
			i += 2
			if i > len(input) {
				i = len(input)
			}
			continue
		}
		if dialect == "postgres" && ch == '$' {
			if end, ok := skipDollar(input, i); ok {
				toks = append(toks, sqlTok{kind: "string"})
				i = end
				continue
			}
		}
		if ch == '\'' || ch == '"' || ch == '`' {
			i = skipSQLString(input, i, ch, dialect)
			toks = append(toks, sqlTok{kind: "string"})
			continue
		}
		if ch == ';' {
			toks = append(toks, sqlTok{kind: "semi"})
			i++
			continue
		}
		if isIdentStart(ch) {
			start := i
			i++
			for i < len(input) && isIdentCont(input[i]) {
				i++
			}
			toks = append(toks, sqlTok{kind: "word", value: strings.ToUpper(input[start:i])})
			continue
		}
		i++
	}
	return toks
}

func isSQLSpace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '\v' || ch == '\f'
}

func isIdentStart(ch byte) bool {
	return ch == '_' || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

func isIdentCont(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9')
}

// $tag$ 里的分号不是第二句；$1 占位符不能当 dollar-quote。
func skipDollar(input string, start int) (int, bool) {
	i := start + 1
	if i < len(input) && isIdentStart(input[i]) {
		i++
		for i < len(input) && isIdentCont(input[i]) {
			i++
		}
	}
	if i >= len(input) || input[i] != '$' {
		return 0, false
	}
	open := input[start : i+1]
	from := i + 1
	closeAt := strings.Index(input[from:], open)
	if closeAt < 0 {
		return len(input), true
	}
	return from + closeAt + len(open), true
}

func skipSQLString(input string, start int, quote byte, dialect string) int {
	i := start + 1
	for i < len(input) {
		ch := input[i]
		if dialect == "mysql" && ch == '\\' && quote != '`' {
			i += 2
			continue
		}
		if ch == quote {
			if i+1 < len(input) && input[i+1] == quote {
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return len(input)
}
