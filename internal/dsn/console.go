package dsn

import (
	"errors"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// ConsoleSpec 是官方客户端的 argv。dsn-cli 只展开 Profile.url，不翻译客户端自己的退出码。
type ConsoleSpec struct {
	Bin  string
	Args []string
}

func consoleSpec(kind, raw string) (ConsoleSpec, error) {
	switch kind {
	case "mysql", "doris":
		return mysqlSpec(raw)
	case "redis":
		return redisSpec(raw)
	case "mongodb":
		return ConsoleSpec{Bin: "mongosh", Args: []string{raw}}, nil
	case "postgres":
		return ConsoleSpec{Bin: "psql", Args: []string{raw}}, nil
	default:
		return ConsoleSpec{}, exitf(1, "kind %s has no Console", kind)
	}
}

func mysqlSpec(raw string) (ConsoleSpec, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ConsoleSpec{}, exitf(1, "invalid url")
	}
	var args []string
	if parsed.Hostname() != "" {
		args = append(args, "-h", parsed.Hostname())
	}
	if parsed.Port() != "" {
		args = append(args, "-P", parsed.Port())
	}
	if parsed.User.Username() != "" {
		args = append(args, "-u", parsed.User.Username())
	}
	if pass, ok := parsed.User.Password(); ok && pass != "" {
		args = append(args, "-p"+pass)
	}
	db := trimSlash(parsed.Path)
	if db != "" {
		args = append(args, db)
	}
	return ConsoleSpec{Bin: "mysql", Args: args}, nil
}

func redisSpec(raw string) (ConsoleSpec, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ConsoleSpec{}, exitf(1, "invalid url")
	}
	var args []string
	if parsed.Hostname() != "" {
		args = append(args, "-h", parsed.Hostname())
	}
	if parsed.Port() != "" {
		args = append(args, "-p", parsed.Port())
	}
	if pass, ok := parsed.User.Password(); ok && pass != "" {
		args = append(args, "-a", pass)
	}
	db := strings.TrimPrefix(parsed.Path, "/")
	if db != "" {
		args = append(args, "-n", db)
	}
	return ConsoleSpec{Bin: "redis-cli", Args: args}, nil
}

func runConsole(kind, raw string) error {
	spec, err := consoleSpec(kind, raw)
	if err != nil {
		return err
	}
	cmd := exec.Command(spec.Bin, spec.Args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err == nil {
		return nil
	}
	var missing *exec.Error
	if errors.As(err, &missing) && errors.Is(missing.Err, exec.ErrNotFound) {
		return exitf(1, "%s not found on PATH", spec.Bin)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &Exit{Code: exitErr.ExitCode()}
	}
	return exitf(3, "%s", err.Error())
}
