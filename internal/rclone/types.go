// Package rclone 实现 rclone-cli。
// 配置只读；empty.conf 和运行记录只进 XDG state，避免写进配置目录。
package rclone

// Source 是本地目录树。路径必须是绝对路径，不能靠当前目录隐式当源。
type Source struct {
	Name string
	Path string
}

// Crypt 是一对内容密钥。加密只用 default，解密才按顺序换对。
type Crypt struct {
	Name      string
	Key       string
	Salt      string
	Algorithm string
	Default   bool
}

// Target 是 WebDAV 目标。账号和传输参数挂在这里，任务只能覆盖少数几项。
type Target struct {
	Name             string
	URL              string
	Vendor           string
	User             string
	Pass             string
	Root             string
	SkipLinks        bool
	SkipHidden       bool
	SizeOnly         bool
	Transfers        float64
	Checkers         float64
	Retries          float64
	RetriesSleep     string
	LowLevelRetries  float64
	Timeout          string
	Contimeout       string
	Tpslimit         float64
	PacerMinSleep    string
	EncryptFilenames bool
	Crypt            []Crypt
	HasCrypt         bool
}

// Task 是唯一可执行的边。没有名字就不能传，也不存在默认任务。
type Task struct {
	Name             string
	Source           string
	Target           string
	Excludes         []string
	Transfers        *float64
	Tpslimit         *float64
	Timeout          *string
	EncryptFilenames *bool
	SkipHidden       *bool
}

// FileCfg 是配置的三层。多出来的 defaultTask 会让缺省命令偷偷开始上传。
type FileCfg struct {
	RcloneBin string
	Sources   []Source
	Targets   []Target
	Tasks     []Task
}

// RunRec 是一次执行的本地记录。记录里不能出现账号和口令。
type RunRec struct {
	ID         string   `json:"id"`
	Task       string   `json:"task"`
	Action     string   `json:"action"`
	StartedAt  string   `json:"startedAt"`
	FinishedAt string   `json:"finishedAt"`
	Code       int      `json:"code"`
	Local      string   `json:"local"`
	Remote     string   `json:"remote"`
	DryRun     bool     `json:"dryRun"`
	Args       []string `json:"args"`
	Bytes      *int64   `json:"bytes,omitempty"`
	Checks     *int64   `json:"checks,omitempty"`
	Transfers  *int64   `json:"transfers,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// SpawnRet 是一次外部 rclone 的结果。单测用它替换真二进制。
type SpawnRet struct {
	Code   int
	Stdout string
	Stderr string
}

// ErrSink 接收 rclone 的 stderr 流。人读模式要边跑边看见，agent 模式必须丢掉进度条。
type ErrSink interface {
	WriteErr(chunk string)
}

// SpawnFn 执行外部 rclone。obscure 和 copy 都走这里，单测不能打到真实网盘。
type SpawnFn func(bin string, args []string, sink ErrSink) (SpawnRet, error)
