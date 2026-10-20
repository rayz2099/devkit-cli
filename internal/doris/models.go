package doris

// FENode 代表前端 FE 节点的元数据与存活心跳状态。
// 各字段保留自 Doris 内核 SHOW FRONTENDS，以供 Agent 判定主备分配与元数据同步延迟。
type FENode struct {
	Name           string `json:"name"`
	IP             string `json:"ip"`
	EditLogPort    int    `json:"edit_log_port"`
	HTTPPort       int    `json:"http_port"`
	QueryPort      int    `json:"query_port"`
	RPCPort        int    `json:"rpc_port"`
	Role           string `json:"role"`
	IsMaster       bool   `json:"is_master"`
	ClusterID      string `json:"cluster_id"`
	Join           bool   `json:"join"`
	Alive          bool   `json:"alive"`
	ReplayedJnlID  string `json:"replayed_journal_id"`
	LastHeartbeat  string `json:"last_heartbeat"`
	IsHelper       bool   `json:"is_helper"`
	ErrMsg         string `json:"err_msg,omitempty"`
	Version        string `json:"version"`
}

// BENode 代表后端 BE 计算与存储节点的状态。
// 存储容量和心跳失败计数器用于让 Agent 识别节点失联或磁盘打满风险。
type BENode struct {
	BackendID      string  `json:"backend_id"`
	IP             string  `json:"ip"`
	HeartbeatPort  int     `json:"heartbeat_port"`
	BEPort         int     `json:"be_port"`
	HTTPPort       int     `json:"http_port"`
	BRPCPort       int     `json:"brpc_port"`
	LastStartTime  string  `json:"last_start_time"`
	LastHeartbeat  string  `json:"last_heartbeat"`
	Alive          bool    `json:"alive"`
	Decommissioned bool    `json:"decommissioned"`
	TabletNum      int64   `json:"tablet_num"`
	DataUsedBytes  int64   `json:"data_used_bytes"`
	AvailCapBytes  int64   `json:"avail_capacity_bytes"`
	TotalCapBytes  int64   `json:"total_capacity_bytes"`
	UsedPct        float64 `json:"used_pct"`
	ErrMsg         string  `json:"err_msg,omitempty"`
	Status         string  `json:"status"`
}

// TabletHealthSummary 汇总全局分片健康度，排查副本丢失或异常。
type TabletHealthSummary struct {
	TotalTablets     int64 `json:"total_tablets"`
	HealthyTablets   int64 `json:"healthy_tablets"`
	UnhealthyTablets int64 `json:"unhealthy_tablets"`
	MissingReplicas  int64 `json:"missing_replicas"`
}

// ClusterReport 是 cluster 命令输出的整体拓扑与健康报告。
type ClusterReport struct {
	MasterFE     string              `json:"master_fe"`
	FETotal      int                 `json:"fe_total"`
	FEAlive      int                 `json:"fe_alive"`
	BETotal      int                 `json:"be_total"`
	BEAlive      int                 `json:"be_alive"`
	Frontends    []FENode            `json:"frontends"`
	Backends     []BENode            `json:"backends"`
	TabletHealth TabletHealthSummary `json:"tablet_health"`
}

// FELogEntry 表示解析出的单条 FE 警告或错误日志。
type FELogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Logger    string `json:"logger"`
	Message   string `json:"message"`
}

// FELogReport 包含从 FE HTTP 端口拉取并过滤后的日志列表。
type FELogReport struct {
	NodeHost     string       `json:"node_host"`
	LogPath      string       `json:"log_path"`
	TotalLines   int          `json:"total_lines"`
	MatchedLines int          `json:"matched_lines"`
	Entries      []FELogEntry `json:"entries"`
}

// FEMetricsReport 包含 FE JVM 运行状态与核心业务指标。
// 提取这些指标用于让 Agent 识别 Full GC 伪掉线和 BDB-JE 事务堆积。
type FEMetricsReport struct {
	NodeHost             string  `json:"node_host"`
	JVMHeapUsedBytes     int64   `json:"jvm_heap_used_bytes"`
	JVMHeapMaxBytes      int64   `json:"jvm_heap_max_bytes"`
	JVMHeapUsedPct       float64 `json:"jvm_heap_used_pct"`
	JVMYoungGCTimeMs     int64   `json:"jvm_young_gc_time_ms"`
	JVMFullGCCount       int64   `json:"jvm_full_gc_count"`
	JVMFullGCTimeMs      int64   `json:"jvm_full_gc_time_ms"`
	ActiveConnections    int64   `json:"active_connections"`
	EditLogWriteLatency  float64 `json:"edit_log_write_latency_ms"`
	QueryTotal           int64   `json:"query_total"`
	QueryErrTotal        int64   `json:"query_err_total"`
	QueryLatencyMs       float64 `json:"query_latency_ms"`
}

// ActiveQueryItem 表示当前在执行或高耗时的查询。
type ActiveQueryItem struct {
	QueryID     string `json:"query_id"`
	User        string `json:"user"`
	ClientIP    string `json:"client_ip"`
	Database    string `json:"database"`
	SQL         string `json:"sql"`
	ExecTimeMs  int64  `json:"exec_time_ms"`
	MemoryBytes int64  `json:"memory_bytes"`
	QueueStatus string `json:"queue_status,omitempty"`
}

// QueryReport 输出活跃与慢查询列表。
type QueryReport struct {
	TotalQueries int               `json:"total_queries"`
	Queries      []ActiveQueryItem `json:"queries"`
}
