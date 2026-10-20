package main

import (
	"encoding/json"
	"io"

	"github.com/spf13/cobra"

	"git.internal.linran.top/linran/devkit-cli/internal/doris"
)

type globalFlags struct {
	configPath string
	profile    string
	host       string
	queryPort  int
	httpPort   int
	user       string
	password   string
	database   string
	output     string
}

var gFlags globalFlags

func execute(args []string, stdout, stderr io.Writer) error {
	root := newRoot()
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)
	return root.Execute()
}

func newRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "doris-cli",
		Short:         "Read-only diagnostic CLI for Apache Doris",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.CompletionOptions.DisableDefaultCmd = true

	// 全局标志
	cmd.PersistentFlags().StringVarP(&gFlags.configPath, "config", "c", "", "Path to config file")
	cmd.PersistentFlags().StringVarP(&gFlags.profile, "profile", "P", "", "Active cluster profile name")
	cmd.PersistentFlags().StringVar(&gFlags.host, "host", "", "Doris FE Host (defaults to env DORIS_HOST or 127.0.0.1)")
	cmd.PersistentFlags().IntVar(&gFlags.queryPort, "query-port", 0, "Doris FE Query Port (defaults to env DORIS_QUERY_PORT or 9030)")
	cmd.PersistentFlags().IntVar(&gFlags.httpPort, "http-port", 0, "Doris FE HTTP Port (defaults to env DORIS_HTTP_PORT or 8030)")
	cmd.PersistentFlags().StringVarP(&gFlags.user, "user", "u", "", "Doris user (defaults to env DORIS_USER or root)")
	cmd.PersistentFlags().StringVarP(&gFlags.password, "password", "p", "", "Doris password (defaults to env DORIS_PASSWORD)")
	cmd.PersistentFlags().StringVarP(&gFlags.database, "database", "d", "", "Doris database")
	cmd.PersistentFlags().StringVar(&gFlags.output, "output", "json", "Output format (json|human)")

	// 注册动态 Shell 补全，按 Tab 键自动提示已配置的 Profiles 列表
	_ = cmd.RegisterFlagCompletionFunc("profile", func(c *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		names := doris.ListProfileNames(gFlags.configPath)
		return names, cobra.ShellCompDirectiveNoFileComp
	})

	// 注册子命令
	cmd.AddCommand(newClusterCmd())
	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newProfilesCmd())
	cmd.AddCommand(newFELogCmd())
	cmd.AddCommand(newFEMetricsCmd())
	cmd.AddCommand(newQueryCmd())
	cmd.AddCommand(newCompletionCmd(cmd))

	return cmd
}

// buildClient 汇总配置文件（指定 profile）、环境变量与命令行参数初始化只读客户端。
func buildClient() (*doris.Client, error) {
	cfg, _, err := doris.LoadConfig(gFlags.configPath, gFlags.profile)
	if err != nil {
		return nil, err
	}

	// 命令行显式指定的参数具有最高优先级
	if gFlags.host != "" {
		cfg.Host = gFlags.host
	}
	if gFlags.queryPort > 0 {
		cfg.QueryPort = gFlags.queryPort
	}
	if gFlags.httpPort > 0 {
		cfg.HTTPPort = gFlags.httpPort
	}
	if gFlags.user != "" {
		cfg.User = gFlags.user
	}
	if gFlags.password != "" {
		cfg.Password = gFlags.password
	}
	if gFlags.database != "" {
		cfg.Database = gFlags.database
	}

	return doris.NewClient(cfg), nil
}

// outputData 统一处理标准输出格式化，默认纯净 JSON 以便 Agent 消费。
func outputData(w io.Writer, val any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(val)
}
