package doctor

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestWriteFailsWhenAnyItemFails(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, []Item{
		{Name: "config", Err: nil},
		{Name: "fields", Err: errors.New("missing kind")},
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	text := buf.String()
	if !strings.Contains(text, "pass config\n") || !strings.Contains(text, "fail fields: missing kind\n") {
		t.Fatalf("output = %q", text)
	}
}
