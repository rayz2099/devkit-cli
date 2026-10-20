package doris

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"git.internal.linran.top/linran/devkit-cli/internal/doctor"
)

// RunDoctor 检查多集群配置完整性与连通性。
// 遵循 devkit-cli 的 doctor 标准契约，输出 pass/fail 列表。
func RunDoctor(w io.Writer, cfgPath, profileName string, ping bool) error {
	var items []doctor.Item

	// 1. 读取配置文件
	fc, err := LoadFileConfig(cfgPath)
	if err != nil {
		items = append(items, doctor.Item{Name: "config", Err: err})
		return doctor.Write(w, items)
	}
	items = append(items, doctor.Item{Name: "config"})

	// 2. 检查 profiles 结构
	if len(fc.Profiles) == 0 {
		items = append(items, doctor.Item{Name: "profiles", Err: errors.New("no profiles configured")})
		return doctor.Write(w, items)
	}

	// 3. 校验各 profile 字段
	if profileName != "" {
		prof, exists := fc.Profiles[profileName]
		if !exists {
			items = append(items, doctor.Item{Name: "profile:" + profileName, Err: fmt.Errorf("profile not found: %s", profileName)})
			return doctor.Write(w, items)
		}
		if fErr := validateClusterFields(prof); fErr != nil {
			items = append(items, doctor.Item{Name: "profile:" + profileName, Err: fErr})
			return doctor.Write(w, items)
		}
		items = append(items, doctor.Item{Name: "profile:" + profileName})
	} else {
		// 校验默认 profile 是否指定且存在
		if fc.DefaultProfile != "" {
			if _, exists := fc.Profiles[fc.DefaultProfile]; !exists {
				items = append(items, doctor.Item{Name: "default_profile", Err: fmt.Errorf("default profile '%s' not found in profiles", fc.DefaultProfile)})
			} else {
				items = append(items, doctor.Item{Name: "default_profile"})
			}
		}

		for name, p := range fc.Profiles {
			if fErr := validateClusterFields(p); fErr != nil {
				items = append(items, doctor.Item{Name: "profile:" + name, Err: fErr})
			} else {
				items = append(items, doctor.Item{Name: "profile:" + name})
			}
		}
	}

	// 4. 可选连通性探测（针对当前生效 profile）
	if ping {
		activeCfg, actName, loadErr := LoadConfig(cfgPath, profileName)
		if loadErr != nil {
			items = append(items, doctor.Item{Name: "ping", Err: loadErr})
			return doctor.Write(w, items)
		}

		cli := NewClient(activeCfg)
		defer cli.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// 探测 FE MySQL 端口
		_, mErr := cli.getDB(ctx)
		items = append(items, doctor.Item{Name: fmt.Sprintf("mysql(%s)", actName), Err: mErr})

		// 探测 FE HTTP 端口
		httpURL := fmt.Sprintf("http://%s:%d/metrics", activeCfg.Host, activeCfg.HTTPPort)
		hReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, httpURL, nil)
		if activeCfg.User != "" {
			hReq.SetBasicAuth(activeCfg.User, activeCfg.Password)
		}
		resp, hErr := cli.hc.Do(hReq)
		if hErr == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 400 && resp.StatusCode != http.StatusUnauthorized {
				hErr = fmt.Errorf("http status %d", resp.StatusCode)
			}
		}
		items = append(items, doctor.Item{Name: fmt.Sprintf("http(%s)", actName), Err: hErr})
	}

	return doctor.Write(w, items)
}

// validateClusterFields 校验具体集群的参数配置。
func validateClusterFields(c ClusterConfig) error {
	if c.Host == "" {
		return errors.New("host cannot be empty")
	}
	if c.QueryPort <= 0 || c.QueryPort > 65535 {
		return fmt.Errorf("invalid query_port: %d", c.QueryPort)
	}
	if c.HTTPPort <= 0 || c.HTTPPort > 65535 {
		return fmt.Errorf("invalid http_port: %d", c.HTTPPort)
	}
	return nil
}
