package olly

import _ "embed"

// 帮助文本和 TypeScript 保持逐字一致。帮助必须在读配置之前就能打印，首次安装才能自检。

//go:embed helptext/HELP_TEXT.txt
var RootHelp string

//go:embed helptext/UPTRACE_HELP_TEXT.txt
var UptraceHelp string

//go:embed helptext/PROMETHEUS_HELP_TEXT.txt
var PrometheusHelp string

//go:embed helptext/GRAYLOG_HELP_TEXT.txt
var GraylogHelp string

//go:embed helptext/GRAFANA_HELP_TEXT.txt
var GrafanaHelp string

// HelpFor 按根子命令选帮助。别名和子命令都归到所属后端，避免每层各写一份。
func HelpFor(name string) string {
	switch name {
	case "uptrace":
		return UptraceHelp
	case "prometheus", "prom":
		return PrometheusHelp
	case "logs", "graylog":
		return GraylogHelp
	case "grafana":
		return GrafanaHelp
	default:
		return RootHelp
	}
}
