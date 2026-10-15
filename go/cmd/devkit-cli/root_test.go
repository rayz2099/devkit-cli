package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestListOmitsAlist(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRoot()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list: %v", err)
	}
	text := buf.String()
	if strings.Contains(text, "alist-cli") {
		t.Fatalf("list contains alist-cli:\n%s", text)
	}
	if !strings.Contains(text, "tl\n") {
		t.Fatalf("list missing tl:\n%s", text)
	}
}

func TestUnknownModule(t *testing.T) {
	cmd := newRoot()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "alist-cli"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error")
	}
}
