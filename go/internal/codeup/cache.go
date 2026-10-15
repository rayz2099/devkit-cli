package codeup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

// CachedRepo 是索引进 agent 补全的本地行。短名只查这里，不再打一次 OpenAPI。
type CachedRepo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	PathNs string `json:"pathNs"`
}

// RepoIndex 是 init 写下的仓库快照。放在 state，是因为配置目录不允许写。
type RepoIndex struct {
	Profile   string       `json:"profile"`
	OrgID     string       `json:"orgId"`
	UpdatedAt string       `json:"updatedAt"`
	Repos     []CachedRepo `json:"repos"`
}

var now = time.Now

// IndexPath 落在 XDG state。路径若会进配置根，直接拒绝，避免 init 改到 ~/.config。
func IndexPath(profile string) (string, error) {
	if strings.TrimSpace(profile) == "" {
		return "", fail(1, "profile is required for repo index")
	}
	if !safeProfile(profile) {
		return "", failf(1, "invalid profile name: %s", profile)
	}
	dirs, err := xdg.Resolve(appName)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dirs.State, "repos-"+profile+".json")
	root, err := xdg.ConfigRoot()
	if err != nil {
		return "", err
	}
	if xdg.Inside(root, path) {
		return "", fmt.Errorf("%w: %s", xdg.ErrInsideConfig, path)
	}
	return path, nil
}

func safeProfile(name string) bool {
	if name == "." || name == ".." || strings.Contains(name, "..") {
		return false
	}
	if strings.ContainsAny(name, `/\`) {
		return false
	}
	return filepath.Base(name) == name
}

// LoadIndex 读取本 profile 的快照。组织和文件对不上就要求重新 init，不用旧索引硬查。
func LoadIndex(profile, orgID string) (RepoIndex, error) {
	path, err := IndexPath(profile)
	if err != nil {
		return RepoIndex{}, err
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RepoIndex{}, failf(1, "repo index not found: %s; run codeup-cli init", path)
		}
		return RepoIndex{}, err
	}
	idx, err := parseIndex(buf, path)
	if err != nil {
		return RepoIndex{}, err
	}
	if idx.OrgID != orgID {
		return RepoIndex{}, fail(1, "repo index org mismatch; run codeup-cli init")
	}
	return idx, nil
}

// SaveIndex 把全量仓库写成快照。只创建 state 目录，不碰配置目录。
func SaveIndex(rt Runtime, repos []Repo) (RepoIndex, error) {
	cached := make([]CachedRepo, 0, len(repos))
	for _, repo := range repos {
		raw := repo.PathNs
		if raw == "" {
			raw = repo.Path
		}
		short, err := shortPath(raw, rt.OrgID)
		if err != nil {
			return RepoIndex{}, err
		}
		cached = append(cached, CachedRepo{ID: repo.ID, Name: repo.Name, PathNs: short})
	}
	idx := RepoIndex{
		Profile:   rt.Profile.Name,
		OrgID:     rt.OrgID,
		UpdatedAt: now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Repos:     cached,
	}
	path, err := IndexPath(rt.Profile.Name)
	if err != nil {
		return RepoIndex{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return RepoIndex{}, err
	}
	text, err := RenderJSON(idx)
	if err != nil {
		return RepoIndex{}, err
	}
	buf := []byte(text)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return RepoIndex{}, err
	}
	return idx, nil
}

// PickCachedRepo 在索引里挑唯一仓库。同分段多个命中必须失败，不能替用户选一个。
func PickCachedRepo(query string, repos []CachedRepo) (CachedRepo, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return CachedRepo{}, fail(1, "repository is required")
	}
	best := -1
	var hits []CachedRepo
	for _, repo := range repos {
		score := ScoreRepo(q, repo)
		if score < 0 {
			continue
		}
		if score > best {
			best = score
			hits = []CachedRepo{repo}
			continue
		}
		if score == best {
			hits = append(hits, repo)
		}
	}
	if len(hits) == 0 {
		return CachedRepo{}, failf(1, "no repository matched %s; run codeup-cli init", query)
	}
	if len(hits) > 1 {
		names := make([]string, len(hits))
		for i, hit := range hits {
			names[i] = hit.PathNs
		}
		return CachedRepo{}, failf(1, "ambiguous repository %s: %s", query, strings.Join(names, ", "))
	}
	return hits[0], nil
}

// ScoreRepo 给短名打分。精确路径优先于子串，避免 demo 命中 demo-api。
func ScoreRepo(query string, repo CachedRepo) int {
	name := strings.ToLower(repo.Name)
	pathNs := strings.ToLower(repo.PathNs)
	if name == query || pathNs == query {
		return 100
	}
	if strings.HasSuffix(pathNs, "/"+query) {
		return 80
	}
	if strings.HasPrefix(name, query) || strings.HasPrefix(pathNs, query) {
		return 60
	}
	if strings.Contains(name, query) || strings.Contains(pathNs, query) {
		return 40
	}
	return -1
}

// HintRepos 把路径和短名都交给补全。只来自索引，补全时不再请求 OpenAPI。
func HintRepos(repos []CachedRepo) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(item string) {
		if _, ok := seen[item]; ok {
			return
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	for _, repo := range repos {
		add(repo.PathNs)
		if repo.Name != repo.PathNs {
			add(repo.Name)
		}
	}
	return out
}

func parseIndex(buf []byte, path string) (RepoIndex, error) {
	var raw any
	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return RepoIndex{}, failf(1, "invalid repo index: %s; run codeup-cli init", path)
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return RepoIndex{}, failf(1, "invalid repo index: %s; run codeup-cli init", path)
	}
	profile := readStr(obj["profile"])
	orgID := readStr(obj["orgId"])
	updated := readStr(obj["updatedAt"])
	if strings.TrimSpace(profile) == "" || strings.TrimSpace(orgID) == "" || strings.TrimSpace(updated) == "" {
		return RepoIndex{}, failf(1, "invalid repo index: %s; run codeup-cli init", path)
	}
	items, ok := obj["repos"].([]any)
	if !ok {
		return RepoIndex{}, failf(1, "invalid repo index: %s; run codeup-cli init", path)
	}
	repos := make([]CachedRepo, 0, len(items))
	for i, item := range items {
		repo, err := parseCached(item, path, i)
		if err != nil {
			return RepoIndex{}, err
		}
		repos = append(repos, repo)
	}
	return RepoIndex{Profile: profile, OrgID: orgID, UpdatedAt: updated, Repos: repos}, nil
}

func parseCached(item any, path string, index int) (CachedRepo, error) {
	obj, ok := item.(map[string]any)
	if !ok {
		return CachedRepo{}, failf(1, "invalid repo index: %s repos[%d]", path, index)
	}
	id := readStr(obj["id"])
	name := readStr(obj["name"])
	pathNs := readStr(obj["pathNs"])
	if id == "" || name == "" || pathNs == "" {
		return CachedRepo{}, failf(1, "invalid repo index: %s repos[%d]", path, index)
	}
	return CachedRepo{ID: id, Name: name, PathNs: pathNs}, nil
}

func shortPath(raw, orgID string) (string, error) {
	parts := splitPath(raw)
	if len(parts) > 0 && parts[0] == orgID {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return "", failf(1, "indexed repository missing path: %s", raw)
	}
	return strings.Join(parts, "/"), nil
}
