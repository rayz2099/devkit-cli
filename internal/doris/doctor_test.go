package doris

import (
	"bytes"
	"strings"
	"testing"
)

// TestDoctorOfflinePass 测试默认离线模式下校验配置。
func TestDoctorOfflinePass(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var buf bytes.Buffer
	err := RunDoctor(&buf, "", "", false)
	if err != nil {
		t.Fatalf("expected doctor to pass, got err: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "pass config") {
		t.Errorf("expected 'pass config', got: %s", out)
	}
	if !strings.Contains(out, "pass profile:default") {
		t.Errorf("expected 'pass profile:default', got: %s", out)
	}
}

// TestDoctorBadConfigPath 测试指定不存在的配置文件报错。
func TestDoctorBadConfigPath(t *testing.T) {
	var buf bytes.Buffer
	err := RunDoctor(&buf, "/path/to/non_existent_config.json", "", false)
	if err == nil {
		t.Fatal("expected doctor to fail for non existent config")
	}

	out := buf.String()
	if !strings.Contains(out, "fail config") {
		t.Errorf("expected 'fail config', got: %s", out)
	}
}
