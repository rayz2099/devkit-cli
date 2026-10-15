package codews

import (
	"fmt"
	"strings"
)

// ParseFrom 解析 --from repo=branch。同一个仓库写两次会让合入目标不确定。
func ParseFrom(raw []string) (map[string]string, error) {
	specs := map[string]string{}
	for _, item := range raw {
		eq := strings.Index(item, "=")
		repo := ""
		ref := ""
		if eq > 0 {
			repo = item[:eq]
			ref = item[eq+1:]
		}
		if repo == "" || ref == "" {
			return nil, fmt.Errorf("--from requires repo=branch: %s", item)
		}
		if _, ok := specs[repo]; ok {
			return nil, fmt.Errorf("--from repeated for repo: %s", repo)
		}
		specs[repo] = ref
	}
	return specs, nil
}

// ApplyMerge 给每个选中的仓库写上 merge。缺省用仓库自己的主分支，agent 不用再猜。
func ApplyMerge(repos []Repo, specs map[string]string, resolve func(Repo, string) (string, error)) ([]Repo, error) {
	names := map[string]bool{}
	for _, repo := range repos {
		names[repo.Name] = true
	}
	for name := range specs {
		if !names[name] {
			return nil, fmt.Errorf("--from repo is not selected: %s", name)
		}
	}
	out := make([]Repo, 0, len(repos))
	for _, repo := range repos {
		spec, ok := specs[repo.Name]
		merge := repo.Merge
		if merge == "" {
			merge = repo.Branch
		}
		if ok {
			resolved, err := resolve(repo, spec)
			if err != nil {
				return nil, err
			}
			merge = resolved
		}
		repo.Merge = merge
		out = append(out, repo)
	}
	return out, nil
}
