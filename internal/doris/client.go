package doris

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Client 封装了对 Doris FE MySQL 与 FE HTTP 的只读访问。
// 之所以将 MySQL 句柄设为懒加载，是因为 Agent 在仅拉取日志或 metrics 时无需强制打通 9030 端口。
type Client struct {
	cfg ClusterConfig
	hc  *http.Client
	db  *sql.DB
}

// NewClient 初始化 Doris 只读客户端。
func NewClient(cfg ClusterConfig) *Client {
	return &Client{
		cfg: cfg,
		hc: &http.Client{
			Timeout: cfg.Timeout(),
		},
	}
}

// Close 释放 MySQL 连接池资源。
func (c *Client) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// getDB 获取或初始化 MySQL 连接池。
func (c *Client) getDB(ctx context.Context) (*sql.DB, error) {
	if c.db != nil {
		return c.db, nil
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=%s&readTimeout=%s",
		c.cfg.User,
		c.cfg.Password,
		c.cfg.Host,
		c.cfg.QueryPort,
		c.cfg.Database,
		c.cfg.Timeout(),
		c.cfg.Timeout(),
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql connection: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(10 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping doris fe mysql (host=%s, port=%d): %w", c.cfg.Host, c.cfg.QueryPort, err)
	}

	c.db = db
	return c.db, nil
}

// scanRowsToMaps 将结果集按列名动态转换为 map 数组。
// 之所以采用动态列映射而非硬编码 Scan，是为了兼容 Doris 不同大版本之间增加列的情况，避免反射 panic。
func scanRowsToMaps(rows *sql.Rows) ([]map[string]string, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var res []map[string]string
	for rows.Next() {
		rawVals := make([]any, len(cols))
		scanArgs := make([]any, len(cols))
		for i := range rawVals {
			scanArgs[i] = &rawVals[i]
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]string, len(cols))
		for i, col := range cols {
			val := rawVals[i]
			if val == nil {
				rowMap[col] = ""
				continue
			}
			switch v := val.(type) {
			case []byte:
				rowMap[col] = string(v)
			default:
				rowMap[col] = fmt.Sprint(v)
			}
		}
		res = append(res, rowMap)
	}
	return res, rows.Err()
}

// GetCluster 获取整个集群的 FE/BE 节点与健康状态。
func (c *Client) GetCluster(ctx context.Context) (*ClusterReport, error) {
	db, err := c.getDB(ctx)
	if err != nil {
		return nil, err
	}

	report := &ClusterReport{}

	// 1. 查询 FE 节点状态
	feRows, err := db.QueryContext(ctx, "SHOW FRONTENDS")
	if err != nil {
		return nil, fmt.Errorf("query SHOW FRONTENDS: %w", err)
	}
	feMaps, err := scanRowsToMaps(feRows)
	_ = feRows.Close()
	if err != nil {
		return nil, fmt.Errorf("parse SHOW FRONTENDS: %w", err)
	}

	for _, m := range feMaps {
		alive := strings.EqualFold(m["Alive"], "true")
		isMaster := strings.EqualFold(m["IsMaster"], "true")
		if isMaster {
			report.MasterFE = m["IP"]
		}
		if alive {
			report.FEAlive++
		}
		report.FETotal++

		editPort, _ := strconv.Atoi(m["EditLogPort"])
		httpPort, _ := strconv.Atoi(m["HttpPort"])
		queryPort, _ := strconv.Atoi(m["QueryPort"])
		rpcPort, _ := strconv.Atoi(m["RpcPort"])

		report.Frontends = append(report.Frontends, FENode{
			Name:          m["Name"],
			IP:            m["IP"],
			EditLogPort:   editPort,
			HTTPPort:      httpPort,
			QueryPort:     queryPort,
			RPCPort:       rpcPort,
			Role:          m["Role"],
			IsMaster:      isMaster,
			ClusterID:     m["ClusterId"],
			Join:          strings.EqualFold(m["Join"], "true"),
			Alive:         alive,
			ReplayedJnlID: m["ReplayedJournalId"],
			LastHeartbeat: m["LastHeartbeat"],
			IsHelper:      strings.EqualFold(m["IsHelper"], "true"),
			ErrMsg:        m["ErrMsg"],
			Version:       m["Version"],
		})
	}

	// 2. 查询 BE 节点状态
	beRows, err := db.QueryContext(ctx, "SHOW BACKENDS")
	if err != nil {
		return nil, fmt.Errorf("query SHOW BACKENDS: %w", err)
	}
	beMaps, err := scanRowsToMaps(beRows)
	_ = beRows.Close()
	if err != nil {
		return nil, fmt.Errorf("parse SHOW BACKENDS: %w", err)
	}

	for _, m := range beMaps {
		alive := strings.EqualFold(m["Alive"], "true")
		if alive {
			report.BEAlive++
		}
		report.BETotal++

		hbPort, _ := strconv.Atoi(m["HeartbeatPort"])
		bePort, _ := strconv.Atoi(m["BePort"])
		httpPort, _ := strconv.Atoi(m["HttpPort"])
		brpcPort, _ := strconv.Atoi(m["BrpcPort"])
		tabNum, _ := strconv.ParseInt(m["TabletNum"], 10, 64)
		usedBytes, _ := strconv.ParseInt(m["DataUsedCapacity"], 10, 64)
		availBytes, _ := strconv.ParseInt(m["AvailCapacity"], 10, 64)
		totBytes, _ := strconv.ParseInt(m["TotalCapacity"], 10, 64)
		usedPctStr := strings.TrimSuffix(m["UsedPct"], "%")
		usedPct, _ := strconv.ParseFloat(usedPctStr, 64)

		decommissioned := strings.EqualFold(m["SystemDecommissioned"], "true") || strings.EqualFold(m["ClusterDecommissioned"], "true")

		report.Backends = append(report.Backends, BENode{
			BackendID:      m["BackendId"],
			IP:             m["IP"],
			HeartbeatPort:  hbPort,
			BEPort:         bePort,
			HTTPPort:       httpPort,
			BRPCPort:       brpcPort,
			LastStartTime:  m["LastStartTime"],
			LastHeartbeat:  m["LastHeartbeat"],
			Alive:          alive,
			Decommissioned: decommissioned,
			TabletNum:      tabNum,
			DataUsedBytes:  usedBytes,
			AvailCapBytes:  availBytes,
			TotalCapBytes:  totBytes,
			UsedPct:        usedPct,
			ErrMsg:         m["ErrMsg"],
			Status:         m["Status"],
		})
	}

	// 3. 查询 Tablet 整体健康状态
	tbRows, err := db.QueryContext(ctx, "SHOW PROC '/cluster_health/tablet_health'")
	if err == nil {
		tbMaps, _ := scanRowsToMaps(tbRows)
		_ = tbRows.Close()
		for _, m := range tbMaps {
			tot, _ := strconv.ParseInt(m["Total"], 10, 64)
			unhealthy, _ := strconv.ParseInt(m["Unhealthy"], 10, 64)
			missing, _ := strconv.ParseInt(m["Missing"], 10, 64)
			report.TabletHealth = TabletHealthSummary{
				TotalTablets:     tot,
				HealthyTablets:   tot - unhealthy,
				UnhealthyTablets: unhealthy,
				MissingReplicas:  missing,
			}
		}
	}

	return report, nil
}

// GetFELog 从 FE 节点的 HTTP 服务获取只读日志切片。
// Doris FE 内置暴露了 /rest/v1/log 接口，直接提供 fe.warn.log 的内容流。
func (c *Client) GetFELog(ctx context.Context, level string, limit int, kw string) (*FELogReport, error) {
	url := fmt.Sprintf("http://%s:%d/rest/v1/log", c.cfg.Host, c.cfg.HTTPPort)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create fe log request: %w", err)
	}

	if c.cfg.User != "" {
		req.SetBasicAuth(c.cfg.User, c.cfg.Password)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch fe log from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fe log request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var rawResp struct {
		Msg  string `json:"msg"`
		Code int    `json:"code"`
		Data struct {
			LogContents struct {
				LogPath string   `json:"logPath"`
				Content []string `json:"content"`
			} `json:"LogContents"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawResp); err != nil {
		return nil, fmt.Errorf("decode fe log json: %w", err)
	}

	contents := rawResp.Data.LogContents.Content
	report := &FELogReport{
		NodeHost:   c.cfg.Host,
		LogPath:    rawResp.Data.LogContents.LogPath,
		TotalLines: len(contents),
	}

	filterLevel := strings.ToUpper(strings.TrimSpace(level))
	filterKw := strings.ToLower(strings.TrimSpace(kw))

	var matched []FELogEntry
	for _, line := range contents {
		tLine := strings.TrimSpace(line)
		if tLine == "" {
			continue
		}

		// 检查关键词
		if filterKw != "" && !strings.Contains(strings.ToLower(tLine), filterKw) {
			continue
		}

		entry := parseLogLine(tLine)
		if filterLevel != "" && filterLevel != "ALL" {
			if !strings.EqualFold(entry.Level, filterLevel) {
				continue
			}
		}

		matched = append(matched, entry)
	}

	report.MatchedLines = len(matched)

	// 截取最新 limit 条
	if limit > 0 && len(matched) > limit {
		matched = matched[len(matched)-limit:]
	}
	report.Entries = matched

	return report, nil
}

// parseLogLine 对典型 Doris log4j / logback 日志行进行轻量切片提取。
func parseLogLine(line string) FELogEntry {
	parts := strings.SplitN(line, " ", 4)
	if len(parts) >= 4 {
		timePart := parts[0] + " " + parts[1]
		lvl := parts[2]
		return FELogEntry{
			Timestamp: timePart,
			Level:     lvl,
			Message:   parts[3],
		}
	}
	return FELogEntry{
		Level:   "UNKNOWN",
		Message: line,
	}
}

// GetFEMetrics 抓取 FE 节点的 Prometheus 监控指标，并提炼核心数值供 Agent 诊断。
func (c *Client) GetFEMetrics(ctx context.Context) (*FEMetricsReport, error) {
	url := fmt.Sprintf("http://%s:%d/metrics", c.cfg.Host, c.cfg.HTTPPort)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create metrics request: %w", err)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch metrics from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metrics request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read metrics response: %w", err)
	}

	report := &FEMetricsReport{
		NodeHost: c.cfg.Host,
	}

	lines := strings.Split(string(body), "\n")
	for _, l := range lines {
		tl := strings.TrimSpace(l)
		if tl == "" || strings.HasPrefix(tl, "#") {
			continue
		}

		fields := strings.Fields(tl)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		valStr := fields[1]

		// 解析堆内存与 GC 指标
		if strings.HasPrefix(name, "jvm_heap_size_bytes{") {
			v, _ := strconv.ParseInt(valStr, 10, 64)
			if strings.Contains(name, `type="used"`) {
				report.JVMHeapUsedBytes = v
			} else if strings.Contains(name, `type="max"`) {
				report.JVMHeapMaxBytes = v
			}
		} else if strings.HasPrefix(name, "jvm_gc_count{") && strings.Contains(name, "G1 Old") {
			v, _ := strconv.ParseInt(valStr, 10, 64)
			report.JVMFullGCCount = v
		} else if strings.HasPrefix(name, "jvm_gc_time_ms{") {
			v, _ := strconv.ParseInt(valStr, 10, 64)
			if strings.Contains(name, "G1 Old") {
				report.JVMFullGCTimeMs = v
			} else {
				report.JVMYoungGCTimeMs += v
			}
		} else if name == "doris_fe_connection_total" || name == "doris_fe_active_connections" {
			v, _ := strconv.ParseInt(valStr, 10, 64)
			report.ActiveConnections = v
		} else if name == "doris_fe_edit_log_write_latency_ms" {
			v, _ := strconv.ParseFloat(valStr, 64)
			report.EditLogWriteLatency = v
		} else if name == "doris_fe_query_total" {
			v, _ := strconv.ParseInt(valStr, 10, 64)
			report.QueryTotal = v
		} else if name == "doris_fe_query_err" {
			v, _ := strconv.ParseInt(valStr, 10, 64)
			report.QueryErrTotal = v
		} else if name == "doris_fe_query_latency_ms" {
			v, _ := strconv.ParseFloat(valStr, 64)
			report.QueryLatencyMs = v
		}
	}

	if report.JVMHeapMaxBytes > 0 {
		report.JVMHeapUsedPct = float64(report.JVMHeapUsedBytes) / float64(report.JVMHeapMaxBytes) * 100.0
	}

	return report, nil
}

// GetQueries 查询集群中正在运行或高耗时的查询。
func (c *Client) GetQueries(ctx context.Context, slowOnly bool, minMs int64) (*QueryReport, error) {
	db, err := c.getDB(ctx)
	if err != nil {
		return nil, err
	}

	// 统一采用 SHOW PROCESSLIST，各 Doris 版本皆稳定可用
	rows, err := db.QueryContext(ctx, "SHOW PROCESSLIST")
	if err != nil {
		return nil, fmt.Errorf("query SHOW PROCESSLIST: %w", err)
	}
	defer rows.Close()

	pMaps, err := scanRowsToMaps(rows)
	if err != nil {
		return nil, fmt.Errorf("parse SHOW PROCESSLIST: %w", err)
	}

	report := &QueryReport{}
	for _, m := range pMaps {
		timeSec, _ := strconv.ParseInt(m["Time"], 10, 64)
		execMs := timeSec * 1000

		// 过滤只看慢查询
		if slowOnly && execMs < minMs {
			continue
		}

		report.Queries = append(report.Queries, ActiveQueryItem{
			QueryID:     m["Id"],
			User:        m["User"],
			ClientIP:    m["Host"],
			Database:    m["Db"],
			SQL:         m["Info"],
			ExecTimeMs:  execMs,
			QueueStatus: m["State"],
		})
	}
	report.TotalQueries = len(report.Queries)

	return report, nil
}
