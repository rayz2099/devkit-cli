package codews

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
)

// 二进制只从 PATH 找。Quick Tunnel 没有可写进配置的身份。
const cloudflaredBin = "cloudflared"

const logCap = 16 * 1024

var (
	tunnelRe = regexp.MustCompile(`(?i)https://([a-z0-9-]+)\.trycloudflare\.com`)
	ansiRe   = regexp.MustCompile("\u001b\\[[0-9;]*m")
)

// QuickArgs 回源固定 127.0.0.1。关掉自动升级，否则进程重启会把这次的随机域名一起丢掉。
func QuickArgs(port int) []string {
	origin := fmt.Sprintf("http://127.0.0.1:%d", port)
	return []string{"tunnel", "--no-autoupdate", "--url", origin}
}

// ExtractTunnel 从 cloudflared 日志取公网地址。api.trycloudflare.com 是申请接口，不是这次的隧道。
func ExtractTunnel(text string) string {
	plain := ansiRe.ReplaceAllString(text, "")
	matches := tunnelRe.FindAllStringSubmatch(plain, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		host := strings.ToLower(match[1])
		if host == "api" {
			continue
		}
		return match[0]
	}
	return ""
}

func appendLog(prev, chunk string) string {
	next := prev + chunk
	if len(next) <= logCap {
		return next
	}
	return next[len(next)-logCap:]
}

// Tunnel 是一次 Quick Tunnel 子进程。Close 只杀子进程，不退出 serve。
type Tunnel struct {
	cmd    *exec.Cmd
	done   chan struct{}
	closed bool
}

// Close 结束 cloudflared。重复调用无效果。
func (t *Tunnel) Close() {
	if t == nil || t.closed {
		return
	}
	t.closed = true
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Signal(syscall.SIGTERM)
	}
}

// StartTunnel 拉起连接器。子进程退出即隧道消失，调用方必须结束 serve。
func StartTunnel(bin string, args []string, onURL func(string), onFatal func(string)) *Tunnel {
	cmd := exec.Command(bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		onFatal(err.Error())
		return &Tunnel{closed: true}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		onFatal(err.Error())
		return &Tunnel{closed: true}
	}
	if err := cmd.Start(); err != nil {
		if isNotFound(err) {
			onFatal(bin + " not found on PATH")
			return &Tunnel{closed: true}
		}
		onFatal(err.Error())
		return &Tunnel{closed: true}
	}
	tun := &Tunnel{cmd: cmd, done: make(chan struct{})}
	var mu sync.Mutex
	log := ""
	url := ""
	take := func(chunk string) {
		mu.Lock()
		log = appendLog(log, chunk)
		if url != "" {
			mu.Unlock()
			return
		}
		found := ExtractTunnel(log)
		if found == "" {
			mu.Unlock()
			return
		}
		url = found
		mu.Unlock()
		onURL(found)
	}
	go readPipe(stdout, take)
	go readPipe(stderr, take)
	go func() {
		defer close(tun.done)
		err := cmd.Wait()
		why := "0"
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				why = fmt.Sprint(exit.ExitCode())
			} else {
				why = err.Error()
			}
		}
		mu.Lock()
		tail := strings.TrimSpace(log)
		got := url
		mu.Unlock()
		if tun.closed {
			return
		}
		if got == "" {
			onFatal(fmt.Sprintf("%s exited (%s) before a tunnel url\n%s", bin, why, tail))
			return
		}
		onFatal(fmt.Sprintf("%s exited (%s)\n%s", bin, why, tail))
	}()
	return tun
}

func readPipe(r io.Reader, take func(string)) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			take(string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if os.IsNotExist(err) {
		return true
	}
	return strings.Contains(err.Error(), "executable file not found") || strings.Contains(err.Error(), "not found")
}

// StartQuick 为已绑定的端口拉起 Quick Tunnel。信号由 serve 收到后再关连接器。
func StartQuick(port int, onURL func(string), onFatal func(string)) *Tunnel {
	return StartTunnel(cloudflaredBin, QuickArgs(port), onURL, onFatal)
}
