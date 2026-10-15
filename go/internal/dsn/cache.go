package dsn

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"

	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

var cacheNameRe = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// topicCachePath 按 profile 分目录。补全只读这份缓存，不能为了补全去打集群。
func topicCachePath(profile string) (string, error) {
	dirs, err := xdg.Resolve("dsn-cli")
	if err != nil {
		return "", err
	}
	safe := cacheNameRe.ReplaceAllString(profile, "_")
	return filepath.Join(dirs.Cache, safe, "topics.json"), nil
}

func readTopicCache(profile string) ([]string, error) {
	path, err := topicCachePath(profile)
	if err != nil {
		return nil, err
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}
	var parsed struct {
		Topics []any `json:"topics"`
	}
	if err := json.Unmarshal(buf, &parsed); err != nil || parsed.Topics == nil {
		return nil, exitf(1, "invalid topic cache %s", path)
	}
	var topics []string
	for _, item := range parsed.Topics {
		text, ok := item.(string)
		if ok {
			topics = append(topics, text)
		}
	}
	if topics == nil {
		topics = []string{}
	}
	return topics, nil
}

func writeTopicCache(profile string, topics []string) error {
	path, err := topicCachePath(profile)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if topics == nil {
		topics = []string{}
	}
	body, err := json.MarshalIndent(struct {
		Topics []string `json:"topics"`
	}{Topics: topics}, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}
