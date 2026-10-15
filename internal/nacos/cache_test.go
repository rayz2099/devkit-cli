package nacos

import (
	"path/filepath"
	"testing"
	"time"
)

func TestListCacheTTL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config-list.json")
	key := cacheKey("127.0.0.1:8848", "public", "")
	items := []configItem{{dataID: "app-rpc", group: "COMMON"}}
	now := time.UnixMilli(1000)
	if err := writeListCache(path, key, items, now); err != nil {
		t.Fatal(err)
	}
	got, ok := readListCache(path, key, now.Add(listTTL-time.Millisecond))
	if !ok || len(got) != 1 || got[0].dataID != "app-rpc" || got[0].group != "COMMON" {
		t.Fatalf("hit = %#v ok=%v", got, ok)
	}
	if _, ok := readListCache(path, key, now.Add(listTTL)); ok {
		t.Fatal("expired cache should miss")
	}
}
