package codews

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

const watchMaxDepth = 64

// Change 是一条归一化后的文件事件。原子保存的删加会被收成 change。
type Change struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

// WatchEvent 是推给浏览器的领域事件，不暴露 fs 通知的平台细节。
type WatchEvent struct {
	Type    string   `json:"type"`
	Changes []Change `json:"changes,omitempty"`
	Status  string   `json:"status,omitempty"`
}

// ReconcileOpts 是一批路径对照文件系统后的输入。
type ReconcileOpts struct {
	Root        string
	Known       map[string]bool
	Paths       map[string]bool
	ForceResync bool
	MaxChanges  int
}

// ScanWatch 建立 watcher 自己的快照。符号链接不跟，避免环。
func ScanWatch(root string) map[string]bool {
	paths := map[string]bool{}
	walkWatch(root, root, paths, 0)
	return paths
}

func walkWatch(root, dir string, paths map[string]bool, depth int) {
	if depth > watchMaxDepth {
		return
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, ent := range ents {
		abs := filepath.Join(dir, ent.Name())
		rel := PosixRel(root, abs)
		if WatchIgnored(rel) {
			continue
		}
		st, err := os.Lstat(abs)
		if err != nil || st.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if !st.IsDir() && !st.Mode().IsRegular() {
			continue
		}
		paths[rel] = true
		if st.IsDir() {
			walkWatch(root, abs, paths, depth+1)
		}
	}
}

func watchable(root, path string) bool {
	abs, err := UnderRoot(root, path)
	if err != nil {
		return false
	}
	st, err := os.Lstat(abs)
	if err != nil || st.Mode()&os.ModeSymlink != 0 {
		return false
	}
	return st.IsDir() || st.Mode().IsRegular()
}

func replaceKnown(target, source map[string]bool) {
	for key := range target {
		delete(target, key)
	}
	for key := range source {
		target[key] = true
	}
}

// ReconcileWatch 在批次结束后再看磁盘。先删后写的保存应收敛成 change，而不是删加两条。
func ReconcileWatch(opts ReconcileOpts) *WatchEvent {
	maxChanges := opts.MaxChanges
	if maxChanges == 0 {
		maxChanges = 1000
	}
	if opts.ForceResync || len(opts.Paths) > maxChanges {
		replaceKnown(opts.Known, ScanWatch(opts.Root))
		return &WatchEvent{Type: "resync"}
	}
	var keys []string
	for path := range opts.Paths {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	var changes []Change
	topology := false
	for _, path := range keys {
		had := opts.Known[path]
		exists := watchable(opts.Root, path)
		if !had && !exists {
			continue
		}
		kind := "create"
		if had {
			if exists {
				kind = "change"
			} else {
				kind = "delete"
			}
		}
		if kind != "change" {
			topology = true
		}
		changes = append(changes, Change{Path: path, Kind: kind})
	}
	if len(changes) == 0 {
		return nil
	}
	if topology {
		replaceKnown(opts.Known, ScanWatch(opts.Root))
	}
	return &WatchEvent{Type: "files-changed", Changes: changes}
}

// WatchCtl 用来停掉轮询。标准库没有递归 fsnotify，轮询对照快照得到同一类事件。
type WatchCtl struct {
	stop chan struct{}
}

// Close 停止监听。重复调用是安全的。
func (c *WatchCtl) Close() {
	if c == nil || c.stop == nil {
		return
	}
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
}

type watchHooks struct {
	Root       string
	OnEvent    func(WatchEvent)
	OnTopology func()
	OnError    func(error)
	Debounce   time.Duration
	MaxChanges int
}

// StartWatch 周期性对照快照。能力缺失时由调用方降级，这里的轮询本身不依赖平台通知。
func StartWatch(hooks watchHooks) *WatchCtl {
	if hooks.Debounce <= 0 {
		hooks.Debounce = 100 * time.Millisecond
	}
	if hooks.MaxChanges == 0 {
		hooks.MaxChanges = 1000
	}
	known := ScanWatch(hooks.Root)
	mtime := watchMtime(hooks.Root, known)
	ctl := &WatchCtl{stop: make(chan struct{})}
	go func() {
		tick := time.NewTicker(hooks.Debounce)
		defer tick.Stop()
		for {
			select {
			case <-ctl.stop:
				return
			case <-tick.C:
				cur := ScanWatch(hooks.Root)
				next := watchMtime(hooks.Root, cur)
				dirty := map[string]bool{}
				for path := range known {
					if !cur[path] || mtime[path] != next[path] {
						dirty[path] = true
					}
				}
				for path := range cur {
					if !known[path] {
						dirty[path] = true
					}
				}
				if len(dirty) == 0 {
					mtime = next
					continue
				}
				event := ReconcileWatch(ReconcileOpts{
					Root:       hooks.Root,
					Known:      known,
					Paths:      dirty,
					MaxChanges: hooks.MaxChanges,
				})
				mtime = watchMtime(hooks.Root, known)
				if event == nil {
					continue
				}
				topo := event.Type == "resync"
				if event.Type == "files-changed" {
					for _, item := range event.Changes {
						if item.Kind != "change" {
							topo = true
						}
					}
				}
				if topo && hooks.OnTopology != nil {
					hooks.OnTopology()
				}
				if hooks.OnEvent != nil {
					hooks.OnEvent(*event)
				}
			}
		}
	}()
	return ctl
}

func watchMtime(root string, paths map[string]bool) map[string]int64 {
	out := map[string]int64{}
	for path := range paths {
		abs, err := UnderRoot(root, path)
		if err != nil {
			continue
		}
		st, err := os.Lstat(abs)
		if err != nil {
			continue
		}
		out[path] = st.ModTime().UnixNano() ^ st.Size()
	}
	return out
}

// UnsupportedWatch 只把明确的能力缺失当成可降级，权限和资源耗尽仍要失败。
func UnsupportedWatch(code string) bool {
	switch code {
	case "ERR_FEATURE_UNAVAILABLE_ON_PLATFORM", "ERR_METHOD_NOT_IMPLEMENTED", "ENOSYS", "ENOTSUP":
		return true
	default:
		return false
	}
}
