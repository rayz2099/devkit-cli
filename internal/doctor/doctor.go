// Package doctor 把本地检查打成稳定的文本。
// 只汇总调用方给的结果，自己不联网，也不规定 JSON 协议。任一项失败就返回错误，调用方用非 0 退出。
package doctor

import (
	"fmt"
	"io"
)

// Item 是一项检查。Err 为 nil 表示通过。
type Item struct {
	Name string
	Err  error
}

// Write 把每项写成一行。失败项带上错误文本，调用方不要把配置原文放进 Err。
func Write(w io.Writer, items []Item) error {
	var failed bool
	for _, item := range items {
		if item.Err != nil {
			failed = true
			if _, err := fmt.Fprintf(w, "fail %s: %s\n", item.Name, item.Err.Error()); err != nil {
				return fmt.Errorf("write doctor: %w", err)
			}
			continue
		}
		if _, err := fmt.Fprintf(w, "pass %s\n", item.Name); err != nil {
			return fmt.Errorf("write doctor: %w", err)
		}
	}
	if failed {
		return fmt.Errorf("doctor failed")
	}
	return nil
}
