package nacos

import (
	"errors"
	"io"

	"git.internal.linran.top/linran/devkit-cli/internal/cfgfile"
	"git.internal.linran.top/linran/devkit-cli/internal/doctor"
)

// runDoctor 只检查现有配置能不能解析、必填字段是否合法。不连 Nacos。
func runDoctor(explicit string, out io.Writer) error {
	path, _, err := configPath(explicit)
	if err != nil {
		return err
	}
	buf, err := cfgfile.Read(path, exampleRel)
	if err != nil {
		return writeDoctor(out, []doctor.Item{{Name: "config", Err: err}})
	}
	file, err := parseFile(buf, path)
	if err != nil {
		return writeDoctor(out, []doctor.Item{{Name: "config", Err: err}})
	}
	if err := validateFile(file); err != nil {
		return writeDoctor(out, []doctor.Item{
			{Name: "config"},
			{Name: "fields", Err: err},
		})
	}
	return writeDoctor(out, []doctor.Item{{Name: "config"}, {Name: "fields"}})
}

func writeDoctor(out io.Writer, items []doctor.Item) error {
	failed := false
	for _, item := range items {
		if item.Err != nil {
			failed = true
		}
	}
	err := doctor.Write(out, items)
	if failed {
		return quietError{}
	}
	return err
}

// quietError 表示失败原因已经写在 stdout，main 不要再往 stderr 补一行。
type quietError struct{}

func (quietError) Error() string { return "doctor failed" }

// Quiet 判断这次失败是否已经在 stdout 说明过。
func Quiet(err error) bool {
	return errors.Is(err, quietError{})
}
