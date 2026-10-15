package rclone

import "fmt"

// Err 是带进程码的失败。
// 用法错误和 rclone 自己的退出码必须分开，调用方才知道有没有真正发起传输。
type Err struct {
	Code int
	Msg  string
}

func (e *Err) Error() string {
	if e == nil {
		return ""
	}
	return e.Msg
}

func fail(code int, format string, args ...any) *Err {
	return &Err{Code: code, Msg: fmt.Sprintf(format, args...)}
}
