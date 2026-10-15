package codews

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var forkEntries = []string{"project.yml", "docs", "spec", "tasks", ".agents"}

const agentsFile = "AGENTS.md"

const contextBody = "# Context\n\nDescribe the current task, target services, and links to PRD files here.\n"

// ApplyAgents 只链接路由说明，避免 workspace 持有一份可被仓库改写的副本。
func ApplyAgents(tplDir, wsDir string) error {
	src := filepath.Join(tplDir, agentsFile)
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("agents template not found: %s", src)
		}
		return err
	}
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(wsDir, agentsFile)
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("workspace template target already exists: %s", dst)
	}
	if err := os.Symlink(src, dst); err != nil {
		return err
	}
	specDir := filepath.Join(wsDir, "spec")
	tasksDir := filepath.Join(wsDir, "tasks")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		return err
	}
	ctx := filepath.Join(specDir, "context.md")
	if _, err := os.Stat(ctx); errors.Is(err, os.ErrNotExist) {
		return os.WriteFile(ctx, []byte(contextBody), 0o644)
	}
	return nil
}

// CopyForkEntries 只复制任务上下文白名单。仓库 worktree 必须由 git 重新创建。
func CopyForkEntries(srcWs, dstWs string) ([]string, error) {
	if err := os.MkdirAll(dstWs, 0o755); err != nil {
		return nil, err
	}
	var copied []string
	for _, entry := range forkEntries {
		src := filepath.Join(srcWs, entry)
		if _, err := os.Lstat(src); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		dst := filepath.Join(dstWs, entry)
		if _, err := os.Lstat(dst); err == nil {
			return nil, fmt.Errorf("workspace fork target already exists: %s", dst)
		}
		if err := copyEntry(src, dst); err != nil {
			return nil, err
		}
		copied = append(copied, entry)
	}
	return copied, nil
}

func copyEntry(src, dst string) error {
	st, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}
	if st.IsDir() {
		if err := os.Mkdir(dst, st.Mode().Perm()); err != nil {
			return err
		}
		ents, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, ent := range ents {
			if err := copyEntry(filepath.Join(src, ent.Name()), filepath.Join(dst, ent.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, st.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
