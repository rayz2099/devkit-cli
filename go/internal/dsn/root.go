package dsn

import (
	"github.com/spf13/cobra"
)

// NewCommand 用 cobra 挂上子命令和标志。真正的 argv 语义在 parseArgs，避免 -p 只能出现在固定位置。
func NewCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "dsn-cli",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.DisableSuggestions = true
	flags := root.PersistentFlags()
	flags.StringP("profile", "p", "", "profile name")
	flags.StringP("config", "c", "", "config file")
	flags.String("output", "", "human query format")
	flags.Bool("pretty", false, "pretty-print json output")
	flags.String("limit", "", "agent row cap")
	flags.String("timeout", "", "query timeout seconds")
	flags.String("connect-timeout", "", "connect timeout seconds")

	add := func(parent *cobra.Command, use string) {
		parent.AddCommand(&cobra.Command{
			Use:           use,
			SilenceUsage:  true,
			SilenceErrors: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				return nil
			},
		})
	}
	for _, name := range []string{"query", "doctor", "ds", "version", "completion"} {
		add(root, name)
	}
	for _, aud := range []string{"agent", "human"} {
		parent := &cobra.Command{
			Use:           aud,
			SilenceUsage:  true,
			SilenceErrors: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				return nil
			},
		}
		for _, name := range []string{"query", "doctor", "ds"} {
			add(parent, name)
		}
		root.AddCommand(parent)
	}
	return root
}
