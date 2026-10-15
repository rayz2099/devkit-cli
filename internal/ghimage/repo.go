package ghimage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// repo 是当前 Git 仓库。repository 和 branch 只有 build 才填，list/add 不访问 GitHub。
type repo struct {
	root          string
	imagesPath    string
	repository    string
	defaultBranch string
}

func resolveLocal(rt runtime) (repo, error) {
	root, err := mustRun(rt, "git", []string{"rev-parse", "--show-toplevel"}, "")
	if err != nil {
		return repo{}, err
	}
	images := filepath.Join(root, "images.yaml")
	if _, err := os.Stat(images); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return repo{}, fmt.Errorf("images.yaml not found at repository root: %s", images)
		}
		return repo{}, err
	}
	return repo{root: root, imagesPath: images}, nil
}

func resolveRepo(rt runtime) (repo, error) {
	local, err := resolveLocal(rt)
	if err != nil {
		return repo{}, err
	}
	raw, err := mustRun(rt, "gh", []string{
		"repo", "view", "--json", "nameWithOwner,defaultBranchRef",
	}, local.root)
	if err != nil {
		return repo{}, err
	}
	var info struct {
		NameWithOwner    string `json:"nameWithOwner"`
		DefaultBranchRef struct {
			Name string `json:"name"`
		} `json:"defaultBranchRef"`
	}
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return repo{}, fmt.Errorf("invalid gh repo json: %w", err)
	}
	if info.NameWithOwner == "" || info.DefaultBranchRef.Name == "" {
		return repo{}, errors.New("gh repo view returned no default branch")
	}
	local.repository = info.NameWithOwner
	local.defaultBranch = info.DefaultBranchRef.Name
	return local, nil
}
