package nacos

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// globals 是一次调用里显式出现过的全局旗标。
// 指针用来区分「没写」和「写成空字符串」：空字符串也要压过环境变量和配置文件。
type globals struct {
	config    *string
	server    *string
	user      *string
	pass      *string
	namespace *string
	output    *string
	dev       bool
}

func (g globals) configValue() string {
	if g.config == nil {
		return ""
	}
	return *g.config
}

type parsedCmd struct {
	flags       map[string]string
	positionals []string
}

func (p parsedCmd) has(name string) bool {
	_, ok := p.flags[name]
	return ok
}

func (p parsedCmd) get(name string) string {
	return p.flags[name]
}

// splitGlobal 把全局旗标从参数里拆出来。
// 位置和子命令前后都能写，和原来的手写 argv 解析一致；等号形式的全局旗标不认。
func splitGlobal(args []string) (globals, []string, error) {
	var g globals
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		token := args[i]
		switch token {
		case "--dev":
			g.dev = true
		case "-c", "--config":
			val, err := readValue(args, i, token)
			if err != nil {
				return globals{}, nil, err
			}
			g.config = &val
			i++
		case "-o", "--output":
			raw, err := readValue(args, i, token)
			if err != nil {
				return globals{}, nil, err
			}
			out, err := readOutput(raw)
			if err != nil {
				return globals{}, nil, err
			}
			g.output = &out
			i++
		case "--server-addr":
			val, err := readValue(args, i, token)
			if err != nil {
				return globals{}, nil, err
			}
			g.server = &val
			i++
		case "--username":
			val, err := readValue(args, i, token)
			if err != nil {
				return globals{}, nil, err
			}
			g.user = &val
			i++
		case "--password":
			val, err := readValue(args, i, token)
			if err != nil {
				return globals{}, nil, err
			}
			g.pass = &val
			i++
		case "-ns", "--namespace":
			val, err := readValue(args, i, token)
			if err != nil {
				return globals{}, nil, err
			}
			g.namespace = &val
			i++
		default:
			if strings.HasPrefix(token, "-o") && len(token) > 2 && !strings.HasPrefix(token, "--") {
				out, err := readOutput(token[2:])
				if err != nil {
					return globals{}, nil, err
				}
				g.output = &out
				continue
			}
			rest = append(rest, token)
		}
	}
	return g, rest, nil
}

// takeCmd 解析子命令自己的旗标。
// 单独出现的长旗标记成 true，这样 --ephemeral 不用再跟一个值。
func takeCmd(args []string) parsedCmd {
	flags := map[string]string{}
	var positionals []string
	for i := 0; i < len(args); i++ {
		token := args[i]
		if token == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if strings.HasPrefix(token, "--") && strings.Contains(token, "=") {
			eq := strings.IndexByte(token, '=')
			flags[token[:eq]] = token[eq+1:]
			continue
		}
		if strings.HasPrefix(token, "--") {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				flags[token] = "true"
				continue
			}
			flags[token] = args[i+1]
			i++
			continue
		}
		positionals = append(positionals, token)
	}
	return parsedCmd{flags: flags, positionals: positionals}
}

func readValue(args []string, index int, flag string) (string, error) {
	if index+1 >= len(args) {
		return "", fmt.Errorf("%s requires a value", flag)
	}
	return args[index+1], nil
}

func readOutput(value string) (string, error) {
	out := strings.ToLower(value)
	if out != "text" && out != "json" {
		return "", fmt.Errorf("invalid output: %s", value)
	}
	return out, nil
}

func hasHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func requireFlag(flags map[string]string, name string) (string, error) {
	val := strings.TrimSpace(flags[name])
	if val == "" {
		return "", fmt.Errorf("%s is required", strings.TrimPrefix(name, "--"))
	}
	return val, nil
}

func requirePort(raw string, ok bool) (int, error) {
	if !ok || strings.TrimSpace(raw) == "" {
		return 0, errPort
	}
	num, good := jsNumber(raw)
	if !good || !isJSInt(num) || num <= 0 {
		return 0, errPort
	}
	return int(num), nil
}

func parseIntFlag(raw string, ok bool, def int, name string) (int, error) {
	if !ok {
		return def, nil
	}
	num, good := jsNumber(raw)
	if !good || !isJSInt(num) || num <= 0 {
		return 0, fmt.Errorf("invalid %s: %s", name, raw)
	}
	return int(num), nil
}

func parseFloatFlag(raw string, ok bool, def float64, name string) (float64, error) {
	if !ok {
		return def, nil
	}
	num, good := jsNumber(raw)
	if !good || math.IsNaN(num) || math.IsInf(num, 0) {
		return 0, fmt.Errorf("invalid %s: %s", name, raw)
	}
	return num, nil
}

func parseBoolFlag(raw string, ok bool, def bool) (bool, error) {
	if !ok {
		return def, nil
	}
	if raw == "true" {
		return true, nil
	}
	if raw == "false" {
		return false, nil
	}
	return false, fmt.Errorf("invalid boolean: %s", raw)
}

func firstNonEmpty(vals ...string) string {
	for _, val := range vals {
		if strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

func at(vals []string, index int) string {
	if index < 0 || index >= len(vals) {
		return ""
	}
	return vals[index]
}

func splitClusters(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

// jsNumber 对齐 JS Number()：空串是 0，两端空白忽略。
func jsNumber(raw string) (float64, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, true
	}
	num, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, false
	}
	return num, true
}

func isJSInt(num float64) bool {
	return !math.IsNaN(num) && !math.IsInf(num, 0) && num == math.Trunc(num)
}

var errPort = fmt.Errorf("port is required")
