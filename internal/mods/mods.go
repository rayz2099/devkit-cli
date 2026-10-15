// Package mods 是根控制面的模块名单。
// 不再扫描 justfile：alist-cli 这次不移植，扫目录会把它又带回来。tl 仍是原来的 Go 工程，只负责转给它自己的 justfile。
package mods

import "fmt"

// Kind 区分「本仓库的 Go 命令」和「原地不动的 tl」。
type Kind int

const (
	// KindGo 用 go build 构建。
	KindGo Kind = iota
	// KindJust 转发到模块自己的 justfile，不改那个工程。
	KindJust
)

// Mod 是根 list/build/install 的一条。
type Mod struct {
	Name string
	Kind Kind
}

// all 按名字排序。改名单时保持有序，list 的输出才能稳定。
var all = []Mod{
	{Name: "code-ws", Kind: KindGo},
	{Name: "codeup-cli", Kind: KindGo},
	{Name: "dsn-cli", Kind: KindGo},
	{Name: "gh-image-cli", Kind: KindGo},
	{Name: "har-cli", Kind: KindGo},
	{Name: "jenkins-cli", Kind: KindGo},
	{Name: "nacos-cli", Kind: KindGo},
	{Name: "olly-cli", Kind: KindGo},
	{Name: "rclone-cli", Kind: KindGo},
	{Name: "td-cli", Kind: KindGo},
	{Name: "tl", Kind: KindJust},
}

// All 返回名单副本，调用方改不到包内切片。
func All() []Mod {
	out := make([]Mod, len(all))
	copy(out, all)
	return out
}

// Select 按名字挑模块。names 为空表示全部。未知名字直接失败，不静默跳过。
func Select(names []string) ([]Mod, error) {
	if len(names) == 0 {
		return All(), nil
	}
	byName := make(map[string]Mod, len(all))
	for _, mod := range all {
		byName[mod.Name] = mod
	}
	out := make([]Mod, 0, len(names))
	for _, name := range names {
		mod, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("module not found: %s", name)
		}
		out = append(out, mod)
	}
	return out, nil
}
