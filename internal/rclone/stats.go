package rclone

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var unit = map[string]float64{
	"B":   1,
	"KiB": 1024,
	"MiB": 1024 * 1024,
	"GiB": 1024 * 1024 * 1024,
	"TiB": 1024 * 1024 * 1024 * 1024,
}

var reBytes = regexp.MustCompile(`(?m)^Transferred:\s+([0-9.]+)\s*([KMGTP]?i?B)\b`)
var reChecks = regexp.MustCompile(`(?m)^Checks:\s+(\d+)\s+/`)
var reXfers = regexp.MustCompile(`(?m)^Transferred:\s+(\d+)\s+/`)

type stats struct {
	bytes     *int64
	checks    *int64
	transfers *int64
}

func parseStats(stderr string) stats {
	out := stats{}
	if match := lastMatch(reBytes, stderr); match != nil {
		qty, err := strconv.ParseFloat(match[1], 64)
		scale, ok := unit[match[2]]
		if err == nil && ok {
			n := int64(math.Round(qty * scale))
			out.bytes = &n
		}
	}
	if match := lastMatch(reChecks, stderr); match != nil {
		if n, err := strconv.ParseInt(match[1], 10, 64); err == nil {
			out.checks = &n
		}
	}
	if match := lastMatch(reXfers, stderr); match != nil {
		if n, err := strconv.ParseInt(match[1], 10, 64); err == nil {
			out.transfers = &n
		}
	}
	return out
}

func lastMatch(re *regexp.Regexp, text string) []string {
	all := re.FindAllStringSubmatch(text, -1)
	if len(all) == 0 {
		return nil
	}
	return all[len(all)-1]
}

// RedactText 把口令换成星号。运行记录和报错会落盘，obscure 之后的值也不能留下。
func RedactText(text string, secrets []string) string {
	sorted := make([]string, 0, len(secrets))
	for _, item := range secrets {
		if item != "" {
			sorted = append(sorted, item)
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})
	out := text
	for _, secret := range sorted {
		out = strings.ReplaceAll(out, secret, "***")
	}
	return out
}

func redactArgs(args []string, label string) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		if strings.HasPrefix(arg, ":webdav") || strings.HasPrefix(arg, ":crypt") {
			out[i] = label
			continue
		}
		out[i] = arg
	}
	return out
}
