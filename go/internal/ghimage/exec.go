package ghimage

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// cmdOut 是一次外部进程的结果。code 非 0 不是 Go error，调用方决定要不要当成失败。
type cmdOut struct {
	stdout string
	stderr string
	code   int
}

// runtime 把进程、时钟和请求号留成接缝。
// build 要轮询 gh，测试不能真的睡满超时。
type runtime struct {
	run   func(bin string, args []string, cwd string) (cmdOut, error)
	now   func() time.Time
	sleep func(time.Duration)
	id    func() (string, error)
}

func defaultRT() runtime {
	return runtime{run: defaultRun, now: time.Now, sleep: time.Sleep, id: newID}
}

func defaultRun(bin string, args []string, cwd string) (cmdOut, error) {
	cmd := exec.Command(bin, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := cmdOut{stdout: stdout.String(), stderr: stderr.String()}
	if err == nil {
		return out, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		out.code = exit.ExitCode()
		return out, nil
	}
	return cmdOut{}, err
}

func mustRun(rt runtime, bin string, args []string, cwd string) (string, error) {
	out, err := rt.run(bin, args, cwd)
	if err != nil {
		return "", err
	}
	if out.code != 0 {
		detail := strings.TrimSpace(out.stderr)
		if detail == "" {
			detail = strings.TrimSpace(out.stdout)
		}
		msg := bin + " " + strings.Join(args, " ") + " failed"
		if detail != "" {
			msg += ": " + detail
		}
		return "", errors.New(msg)
	}
	return strings.TrimSpace(out.stdout), nil
}

// newID 生成请求号。它会进 Action 的 run-name，并发构建同一个 tag 时才能对上这一次。
func newID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("request id: %w", err)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(buf[:])
	return raw[0:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:32], nil
}
