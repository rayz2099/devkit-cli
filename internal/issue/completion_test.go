package issue

import (
	"bytes"
	"strings"
	"testing"
)

// TestCompletionOffline 保证补全不会因个人配置缺失而失败，更不会联网或泄露凭据。
func TestCompletionOffline(t *testing.T) {
	for _, args := range [][]string{
		{"completion", "fish"},
		{"__complete", "--audience", ""},
		{"__complete", "me", ""},
	} {
		var out, stderr bytes.Buffer
		opts := Options{Out: &out, Err: &stderr, Load: func(string) (Config, error) {
			t.Fatal("completion must not load credentials")
			return Config{}, nil
		}}
		if code := Execute(args, opts); code != 0 {
			t.Fatalf("%v: code=%d stderr=%s", args, code, stderr.String())
		}
		if out.Len() == 0 {
			t.Fatalf("%v: empty completion", args)
		}
		if len(args) > 1 && args[1] == "--audience" && !strings.Contains(out.String(), "agent") {
			t.Fatalf("audience choices missing: %s", out.String())
		}
	}
}
