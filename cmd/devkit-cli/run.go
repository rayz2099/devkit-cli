package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"git.internal.linran.top/linran/devkit-cli/internal/install"
	"git.internal.linran.top/linran/devkit-cli/internal/mods"
)

func selected(names []string) ([]mods.Mod, error) {
	return mods.Select(names)
}

// findRoot 从当前目录和可执行文件向上找仓库。
// 控制面要编译仓库里的命令，装到别处之后也得能回到源码树。
func findRoot() (string, error) {
	var starts []string
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		dir := start
		for {
			tl := filepath.Join(dir, "tl", "justfile")
			gomod := filepath.Join(dir, "go.mod")
			if fileExists(tl) && fileExists(gomod) {
				return dir, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return "", fmt.Errorf("repo root not found")
}

func runTask(root, dest, task string, mod mods.Mod) error {
	switch mod.Kind {
	case mods.KindJust:
		return runJust(root, task, mod.Name)
	case mods.KindGo:
		return runGo(root, dest, task, mod.Name)
	default:
		return fmt.Errorf("unknown module kind: %s", mod.Name)
	}
}

func runJust(root, task, name string) error {
	cmd := exec.Command("just", task)
	cmd.Dir = filepath.Join(root, name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", task, name, err)
	}
	fmt.Printf("%s %s\n", task, name)
	return nil
}

func runGo(root, dest, task, name string) error {
	switch task {
	case "clean":
		path := filepath.Join(root, "bin", name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clean %s: %w", name, err)
		}
	case "build":
		if err := buildGo(root, name); err != nil {
			return err
		}
	case "install":
		if err := buildGo(root, name); err != nil {
			return err
		}
		src := filepath.Join(root, "bin", name)
		if _, err := install.Place(src, dest); err != nil {
			return fmt.Errorf("install %s: %w", name, err)
		}
	default:
		return fmt.Errorf("unknown task: %s", task)
	}
	fmt.Printf("%s %s\n", task, name)
	return nil
}

// buildGo 关闭 cgo，避免二进制绑上本机的 C 库。
func buildGo(root, name string) error {
	out := filepath.Join(root, "bin", name)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("mkdir bin: %w", err)
	}
	cmd := exec.Command("go", "build", "-o", out, "./cmd/"+name)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build %s: %w", name, err)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
