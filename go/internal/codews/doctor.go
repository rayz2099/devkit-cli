package codews

import (
	"io"

	"git.internal.linran.top/linran/devkit-cli/internal/doctor"
)

// runDoctor 只检查配置文件能解析、必填字段合法。不联网，也不碰仓库目录。
func runDoctor(out io.Writer, flag string) error {
	path, err := ConfigPath(flag)
	if err != nil {
		return err
	}
	_, err = LoadRaw(path)
	items := []doctor.Item{{Name: "config", Err: err}}
	if werr := doctor.Write(out, items); werr != nil {
		return &Exit{Code: 1}
	}
	return nil
}
