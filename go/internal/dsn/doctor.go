package dsn

import (
	"fmt"
	"io"
	"strings"

	"git.internal.linran.top/linran/devkit-cli/internal/doctor"
)

// RunDoctor 只检查配置文件本身。旧的联网探测已经去掉，这里不拨号。
func RunDoctor(w io.Writer, path, profile string) error {
	file, err := Load(path)
	if err != nil {
		return doctor.Write(w, []doctor.Item{{Name: "config", Err: err}})
	}
	items := []doctor.Item{{Name: "config"}}
	matched := profile == ""
	for _, prob := range file.Problems {
		if profile != "" && prob.Name != profile {
			continue
		}
		if profile != "" && prob.Name == profile {
			matched = true
		}
		items = append(items, doctor.Item{Name: prob.Name, Err: cleanErr(prob.Err)})
	}
	for _, prof := range file.Profiles {
		if profile != "" && prof.Name != profile {
			continue
		}
		if profile != "" && prof.Name == profile {
			matched = true
		}
		items = append(items, doctor.Item{Name: prof.Name})
	}
	if profile != "" && !matched {
		items = append(items, doctor.Item{Name: profile, Err: fmt.Errorf("profile not found: %s", profile)})
	}
	return doctor.Write(w, items)
}

func cleanErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if i := strings.Index(msg, ": "); i >= 0 && strings.Contains(msg[:i], "invalid config file") {
		return fmt.Errorf("%s", msg[i+2:])
	}
	return err
}
