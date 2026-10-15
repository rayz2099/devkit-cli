package jenkins

import (
	"bytes"
	"encoding/json"
	"errors"
)

// CodeErr 携带进程退出码。404 和 API 失败必须跟旧 CLI 一样分别是 2 和 3，不能全变成 1。
type CodeErr struct {
	Code int
	Msg  string
}

func (e *CodeErr) Error() string { return e.Msg }

func codeErr(code int, msg string) error {
	return &CodeErr{Code: code, Msg: msg}
}

// ExitCode 把错误翻成进程码。普通错误是 1，CodeErr 保留自己的码。
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ce *CodeErr
	if errors.As(err, &ce) {
		return ce.Code
	}
	return 1
}

// Profile 是一份 Jenkins 连接。密钥明文在配置文件里，这是这个 CLI 的存储约定。
type Profile struct {
	Name     string
	URL      string
	User     string
	Password string
	APIToken string
}

// FileCfg 是配置文件。defaultProfile 必须指向 profiles 里的一个名字，运行时再选。
type FileCfg struct {
	Default  string
	Profiles []Profile
}

// JobRef 是目录列表里的一条。className 用驼峰，因为 JSON 给 agent，不暴露 Jenkins 的 _class。
type JobRef struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	ClassName string `json:"className"`
}

// JobView 是 job view 的字段。数字用指针，是为了把「没有这次构建」写成 null 而不是 0。
type JobView struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	Buildable  bool   `json:"buildable"`
	Last       *int64 `json:"lastBuild"`
	LastOK     *int64 `json:"lastSuccessfulBuild"`
	LastFailed *int64 `json:"lastFailedBuild"`
}

// RunItem 是 run ls 的一行。
type RunItem struct {
	Number     int64  `json:"number"`
	Result     string `json:"result"`
	Building   bool   `json:"building"`
	Timestamp  int64  `json:"timestamp"`
	DurationMs int64  `json:"durationMs"`
}

// RunStatus 是精简状态。displayName 缺省就省略，空字符串仍要保留。
type RunStatus struct {
	Job         string  `json:"job"`
	Build       int64   `json:"build"`
	Result      string  `json:"result"`
	Building    bool    `json:"building"`
	DurationMs  int64   `json:"durationMs"`
	Timestamp   int64   `json:"timestamp"`
	URL         string  `json:"url"`
	DisplayName *string `json:"displayName,omitempty"`
}

// Artifact 是构建产物。
type Artifact struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// RunInfo 是 run view 的完整对象。嵌入 RunStatus 是为了 JSON 字段顺序跟旧 CLI 一致。
type RunInfo struct {
	RunStatus
	EstimatedMs int64      `json:"estimatedDurationMs"`
	Description *string    `json:"description"`
	Artifacts   []Artifact `json:"artifacts"`
	Causes      []string   `json:"causes"`
	Params      paramMap   `json:"params"`
}

// TriggerOut 是触发结果。queueLocation 没有时必须是 null，agent 靠这个字段判断队列地址。
type TriggerOut struct {
	Job           string  `json:"job"`
	Triggered     bool    `json:"triggered"`
	Status        int     `json:"status"`
	QueueLocation *string `json:"queueLocation"`
}

// QueueItem 是队列里的一条。
type QueueItem struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Why   string `json:"why"`
	Since int64  `json:"inQueueSince"`
}

// CancelOut 是取消请求的回执。Jenkins 只接受 stop，不保证已经停下来。
type CancelOut struct {
	JobPath string `json:"jobPath"`
	Build   string `json:"build"`
	Action  string `json:"action"`
	Status  string `json:"status"`
}

// paramMap 保住参数插入顺序。encoding/json 的 map 会按字母排序，agent 看到的顺序会变。
type paramMap struct {
	keys []string
	val  map[string]string
}

func (m *paramMap) set(k, v string) {
	if m.val == nil {
		m.val = map[string]string{}
	}
	if _, ok := m.val[k]; !ok {
		m.keys = append(m.keys, k)
	}
	m.val[k] = v
}

func (m paramMap) MarshalJSON() ([]byte, error) {
	if len(m.keys) == 0 {
		return []byte("{}"), nil
	}
	var buf bytes.Buffer
	buf.WriteString("{\n")
	for i, k := range m.keys {
		kb, err := marshalStr(k)
		if err != nil {
			return nil, err
		}
		vb, err := marshalStr(m.val[k])
		if err != nil {
			return nil, err
		}
		buf.WriteString("    ")
		buf.Write(kb)
		buf.WriteString(": ")
		buf.Write(vb)
		if i != len(m.keys)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("  }")
	return buf.Bytes(), nil
}

func marshalStr(s string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
