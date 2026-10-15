package codeup

import "fmt"

// ExitErr 把进程码和文案绑在一起。
// main 只在 RunE 之外退出，调用方靠数字区分用法错误和 API 错误。
type ExitErr struct {
	Code   int
	Msg    string
	Silent bool
}

func (e *ExitErr) Error() string {
	if e == nil {
		return ""
	}
	return e.Msg
}

func fail(code int, msg string) error {
	return &ExitErr{Code: code, Msg: msg}
}

func failf(code int, format string, args ...any) error {
	return fail(code, fmt.Sprintf(format, args...))
}
