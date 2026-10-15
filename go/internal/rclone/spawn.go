package rclone

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strings"
)

type writerSink struct {
	w io.Writer
}

func (s writerSink) WriteErr(chunk string) {
	if s.w == nil || chunk == "" {
		return
	}
	_, _ = io.WriteString(s.w, chunk)
}

type noop struct{}

func (noop) WriteErr(string) {}

func noopSink() ErrSink { return noop{} }

func stderrSink(w io.Writer) ErrSink { return writerSink{w: w} }

// SpawnProc 启动外部 rclone。进度在 stderr 上是流，收完再打印会让人看不到传输还在跑。
func SpawnProc(bin string, args []string, sink ErrSink) (SpawnRet, error) {
	cmd := exec.Command(bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return SpawnRet{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return SpawnRet{}, err
	}
	if err := cmd.Start(); err != nil {
		return SpawnRet{}, err
	}
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	outErr := make(chan error, 1)
	errErr := make(chan error, 1)
	go func() {
		_, err := io.Copy(&outBuf, stdout)
		outErr <- err
	}()
	go func() {
		errErr <- copyErr(&errBuf, stderr, sink)
	}()
	if err := <-outErr; err != nil {
		return SpawnRet{}, err
	}
	if err := <-errErr; err != nil {
		return SpawnRet{}, err
	}
	waitErr := cmd.Wait()
	code := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			return SpawnRet{}, waitErr
		}
	}
	return SpawnRet{Code: code, Stdout: outBuf.String(), Stderr: errBuf.String()}, nil
}

func copyErr(dst *bytes.Buffer, src io.Reader, sink ErrSink) error {
	buf := make([]byte, 4096)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			dst.WriteString(chunk)
			if sink != nil {
				sink.WriteErr(chunk)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func obscureArgs(cfgFile, plain string) []string {
	return []string{"--config", cfgFile, "obscure", "--", plain}
}

func obscureOne(bin, cfgFile, plain string, spawn SpawnFn) (string, error) {
	ret, err := spawn(bin, obscureArgs(cfgFile, plain), noopSink())
	if err != nil {
		return "", err
	}
	if ret.Code != 0 {
		return "", fail(1, "rclone obscure failed (%d)", ret.Code)
	}
	line := strings.TrimSpace(ret.Stdout)
	if line == "" {
		return "", fail(1, "rclone obscure returned empty")
	}
	return line, nil
}

func obscureTarget(bin, cfgFile string, target Target, spawn SpawnFn) (Secrets, error) {
	pass, err := obscureOne(bin, cfgFile, target.Pass, spawn)
	if err != nil {
		return Secrets{}, err
	}
	if !target.HasCrypt {
		return Secrets{Pass: pass}, nil
	}
	var crypt []CryptSecret
	for _, pair := range target.Crypt {
		password, err := obscureOne(bin, cfgFile, pair.Key, spawn)
		if err != nil {
			return Secrets{}, err
		}
		password2, err := obscureOne(bin, cfgFile, pair.Salt, spawn)
		if err != nil {
			return Secrets{}, err
		}
		crypt = append(crypt, CryptSecret{Name: pair.Name, Password: password, Password2: password2})
	}
	return Secrets{Pass: pass, Crypt: crypt}, nil
}
