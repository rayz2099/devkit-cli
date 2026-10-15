package rclone

import (
	"io"

	"git.internal.linran.top/linran/devkit-cli/internal/doctor"
)

// runDoctor 只检查配置能解析、必填字段合法。不连网，也不调用 rclone。
func runDoctor(w io.Writer, home, cfgFlag string) error {
	err := checkConfig(home, cfgFlag)
	items := []doctor.Item{{Name: "config", Err: err}}
	if werr := doctor.Write(w, items); werr != nil && err == nil {
		return werr
	}
	if err != nil {
		return &Err{Code: 1}
	}
	return nil
}

func checkConfig(home, cfgFlag string) error {
	if home == "" {
		return fail(1, "HOME is required")
	}
	path, err := cfgFile(cfgFlag)
	if err != nil {
		return err
	}
	_, err = Load(path, home)
	return err
}
