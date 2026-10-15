package nacos

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const listTTL = 10 * time.Second

type cacheItem struct {
	DataID string `json:"dataId"`
	Group  string `json:"group"`
}

type cacheEnt struct {
	Expire int64       `json:"expire"`
	Items  []cacheItem `json:"items"`
}

type cacheFile struct {
	Entries map[string]cacheEnt `json:"entries"`
}

func cacheKey(server, namespace, user string) string {
	return server + "|" + namespace + "|" + user
}

// readListCache 给每次 Tab 新进程复用 10 秒内的配置列表。
// 文件坏了或过期就当没有，补全不能因为缓存损坏而失败。
func readListCache(path, key string, now time.Time) ([]configItem, bool) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var parsed cacheFile
	if err := json.Unmarshal(buf, &parsed); err != nil || parsed.Entries == nil {
		return nil, false
	}
	ent, ok := parsed.Entries[key]
	if !ok || now.UnixMilli() >= ent.Expire || ent.Items == nil {
		return nil, false
	}
	out := make([]configItem, 0, len(ent.Items))
	for _, item := range ent.Items {
		out = append(out, configItem{dataID: item.DataID, group: item.Group})
	}
	return out, true
}

func writeListCache(path, key string, items []configItem, now time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	parsed := cacheFile{Entries: map[string]cacheEnt{}}
	if buf, err := os.ReadFile(path); err == nil {
		var current cacheFile
		if json.Unmarshal(buf, &current) == nil && current.Entries != nil {
			parsed = current
		}
	}
	next := cacheFile{Entries: map[string]cacheEnt{}}
	stamp := now.UnixMilli()
	for entryKey, ent := range parsed.Entries {
		if ent.Expire > stamp {
			next.Entries[entryKey] = ent
		}
	}
	stored := make([]cacheItem, 0, len(items))
	for _, item := range items {
		stored = append(stored, cacheItem{DataID: item.dataID, Group: item.group})
	}
	next.Entries[key] = cacheEnt{
		Expire: stamp + listTTL.Milliseconds(),
		Items:  stored,
	}
	buf, err := marshalCompact(next)
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0o644)
}

func marshalCompact(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
