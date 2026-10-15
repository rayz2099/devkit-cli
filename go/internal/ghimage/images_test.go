package ghimage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewVersionMovesFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "images.yaml")
	text := "# inventory\nimages:\n  cpa:\n    type: mirror\n    source: eceasy/cli-proxy-api\n    versions:\n      latest:\n        status: done\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	book, err := loadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := book.addMirror("eceasy/cli-proxy-api", "cpa", "v7.2.130"); err != nil {
		t.Fatal(err)
	}
	if err := book.save(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(got)
	if !strings.HasPrefix(body, "# inventory") {
		t.Fatalf("comment lost: %s", body)
	}
	if strings.Index(body, "v7.2.130") < 0 || strings.Index(body, "v7.2.130") > strings.Index(body, "latest") {
		t.Fatalf("version order: %s", body)
	}
	again, err := loadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	vers, err := again.versions("cpa")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(vers, ",") != "v7.2.130,latest" {
		t.Fatalf("versions: %#v", vers)
	}
}

func TestAliasKeepsSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "images.yaml")
	if err := os.WriteFile(path, []byte("images: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	book, err := loadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := book.addMirror("postgres", "postgres", "latest"); err != nil {
		t.Fatal(err)
	}
	err = book.addMirror("other/postgres", "postgres", "v2")
	if err == nil || !strings.Contains(err.Error(), "already maps to a different image source") {
		t.Fatalf("got %v", err)
	}
}

func TestSplitSourceUsesLastSegment(t *testing.T) {
	source, version, ok, err := splitSource("localhost:5000/team/app:v1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || source != "localhost:5000/team/app" || version != "v1" {
		t.Fatalf("got %s %s %v", source, version, ok)
	}
}

func TestSplitSourceRejectsDigest(t *testing.T) {
	_, _, _, err := splitSource("postgres@sha256:abcd")
	if err == nil || !strings.Contains(err.Error(), "digest references are not supported") {
		t.Fatalf("got %v", err)
	}
}

func TestStatusUpdateKeepsOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "images.yaml")
	text := "images:\n  cpa:\n    type: mirror\n    source: postgres\n    versions:\n      v1:\n        status: init\n      latest:\n        status: done\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	book, err := loadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := book.setStatus("cpa", "v1", "done", false); err != nil {
		t.Fatal(err)
	}
	vers, err := book.versions("cpa")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(vers, ",") != "v1,latest" {
		t.Fatalf("versions: %#v", vers)
	}
}

func TestDockerfileRejectsParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "images.yaml")
	if err := os.WriteFile(path, []byte("images: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	book, err := loadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	err = book.addDockerfile("dockerfile/../build.sh", "sail", "0.6.4")
	if err == nil || !strings.Contains(err.Error(), "without '..'") {
		t.Fatalf("got %v", err)
	}
}
