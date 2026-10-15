package dsn

import "fmt"

// Exit 把用法、门禁和驱动失败分成不同进程码，agent 才能分清是谁拒的。
type Exit struct {
	Code int
	Msg  string
}

func (e *Exit) Error() string {
	if e == nil {
		return ""
	}
	return e.Msg
}

func exitf(code int, format string, args ...any) *Exit {
	return &Exit{Code: code, Msg: fmt.Sprintf(format, args...)}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if e, ok := err.(*Exit); ok && e.Code != 0 {
		return e.Code
	}
	return 1
}

func asExit(err error) *Exit {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Exit); ok {
		return e
	}
	return nil
}
