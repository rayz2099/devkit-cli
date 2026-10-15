// Package install 把已经构建好的二进制放到目标目录。
// 不写配置、不装补全。目标若落在 XDG 配置根里就拒绝。已有文件先备份到临时目录，再原子替换。
package install

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

// Place 把 src 安装到 destDir。返回最终路径。
func Place(src, destDir string) (string, error) {
	if os.Geteuid() == 0 {
		return "", fmt.Errorf("refusing to install as root")
	}
	cfgRoot, err := xdg.ConfigRoot()
	if err != nil {
		return "", err
	}
	if xdg.Inside(cfgRoot, destDir) {
		return "", fmt.Errorf("refusing install into config dir: %s", destDir)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir dest: %w", err)
	}
	dest := filepath.Join(destDir, filepath.Base(src))
	if _, err := os.Stat(dest); err == nil {
		if err := backup(dest); err != nil {
			return "", err
		}
	}
	tmp := dest + ".tmp"
	if err := copyFile(src, tmp); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return "", fmt.Errorf("chmod: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", fmt.Errorf("rename install: %w", err)
	}
	return dest, nil
}

func backup(path string) error {
	name := filepath.Base(path) + "-" + time.Now().Format("20060102T150405")
	dst := filepath.Join(os.TempDir(), "devkit-backup-"+name)
	if err := copyFile(path, dst); err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	fmt.Fprintf(os.Stderr, "backup %s -> %s\n", path, dst)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return out.Close()
}
