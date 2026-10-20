package doris

import (
	"os"
	"strings"
	"testing"
)

// TestParseLogLine 测试 FE 日志文本切片解析。
func TestParseLogLine(t *testing.T) {
	line := "2026-10-20 10:00:00,123 WARN (main|1) [org.apache.doris.catalog.Catalog] master election failed"
	entry := parseLogLine(line)

	if entry.Timestamp != "2026-10-20 10:00:00,123" {
		t.Errorf("expected timestamp '2026-10-20 10:00:00,123', got '%s'", entry.Timestamp)
	}
	if entry.Level != "WARN" {
		t.Errorf("expected level 'WARN', got '%s'", entry.Level)
	}
	if !strings.Contains(entry.Message, "master election failed") {
		t.Errorf("expected message to contain 'master election failed', got '%s'", entry.Message)
	}
}

// TestLoadConfigEnv 测试环境变量加载与覆盖。
func TestLoadConfigEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	os.Setenv("DORIS_HOST", "10.0.32.42")
	os.Setenv("DORIS_QUERY_PORT", "9031")
	os.Setenv("DORIS_HTTP_PORT", "8031")
	os.Setenv("DORIS_USER", "admin")
	defer func() {
		os.Unsetenv("DORIS_HOST")
		os.Unsetenv("DORIS_QUERY_PORT")
		os.Unsetenv("DORIS_HTTP_PORT")
		os.Unsetenv("DORIS_USER")
	}()

	cfg, name, err := LoadConfig("", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if name != "default" {
		t.Errorf("expected default profile name, got %s", name)
	}
	if cfg.Host != "10.0.32.42" {
		t.Errorf("expected host 10.0.32.42, got %s", cfg.Host)
	}
	if cfg.QueryPort != 9031 {
		t.Errorf("expected query port 9031, got %d", cfg.QueryPort)
	}
	if cfg.HTTPPort != 8031 {
		t.Errorf("expected http port 8031, got %d", cfg.HTTPPort)
	}
	if cfg.User != "admin" {
		t.Errorf("expected user admin, got %s", cfg.User)
	}
}
