// Package cfgfile 只读配置文件。
// 缺文件时报错并带上 example 路径，由用户自己复制。这里不创建、不覆盖、不改权限。
package cfgfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"unicode/utf8"
)

// ErrMissing 表示配置文件不存在。doctor 和业务命令都用它区分「没有配置」和「配置坏了」。
var ErrMissing = errors.New("config missing")

// Read 读取配置文件内容。example 是仓库里的示例相对路径，写进错误里让用户知道该复制哪一份。
func Read(path, example string) ([]byte, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s (copy from %s)", ErrMissing, path, example)
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	return buf, nil
}

// ParseJSON 按严格 JSON 解析。调用方负责字段校验，这里不填默认业务值。
func ParseJSON(buf []byte, dest any) error {
	if err := json.Unmarshal(buf, dest); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	return nil
}

// ParseJSONC 在解析前去掉注释和尾逗号。
// dsn-cli 与 rclone-cli 的手写配置允许这两样，不能逼用户先改成严格 JSON。
func ParseJSONC(buf []byte, dest any) error {
	return ParseJSON([]byte(stripJSONC(string(buf))), dest)
}

// stripJSONC 对齐 dsn-cli 的 stripJsonc：字符串内的斜杠保留，块注释和行注释丢掉，对象或数组结束前的逗号丢掉。
func stripJSONC(text string) string {
	var out []byte
	inStr := false
	escape := false
	for i := 0; i < len(text); {
		ch, size := utf8.DecodeRuneInString(text[i:])
		next, nextSize := utf8.DecodeRuneInString(text[i+size:])
		if inStr {
			out = appendRune(out, ch)
			if escape {
				escape = false
			} else if ch == '\\' {
				escape = true
			} else if ch == '"' {
				inStr = false
			}
			i += size
			continue
		}
		if ch == '"' {
			inStr = true
			out = appendRune(out, ch)
			i += size
			continue
		}
		if ch == '/' && next == '/' {
			i += size + nextSize
			for i < len(text) {
				nl, n := utf8.DecodeRuneInString(text[i:])
				if nl == '\n' {
					break
				}
				i += n
			}
			continue
		}
		if ch == '/' && next == '*' {
			i += size + nextSize
			for i < len(text) {
				a, n := utf8.DecodeRuneInString(text[i:])
				b, _ := utf8.DecodeRuneInString(text[i+n:])
				if a == '*' && b == '/' {
					i += n + utf8.RuneLen(b)
					break
				}
				i += n
			}
			continue
		}
		if ch == ',' {
			look := i + size
			for look < len(text) {
				sp, n := utf8.DecodeRuneInString(text[look:])
				if sp != ' ' && sp != '\t' && sp != '\n' && sp != '\r' {
					break
				}
				look += n
			}
			end, _ := utf8.DecodeRuneInString(text[look:])
			if end == '}' || end == ']' {
				i += size
				continue
			}
		}
		out = appendRune(out, ch)
		i += size
	}
	return string(out)
}

func appendRune(buf []byte, ch rune) []byte {
	var tmp [utf8.UTFMax]byte
	n := utf8.EncodeRune(tmp[:], ch)
	return append(buf, tmp[:n]...)
}
