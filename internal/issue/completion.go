package issue

import "github.com/spf13/cobra"

// configureCompletion 补全仅提供本地命令与参数，避免按 Tab 时读取凭据或访问服务。
func configureCompletion(root *cobra.Command) {
	audience := cobra.FixedCompletions([]string{"human\t易读文本", "agent\t结构化 JSON"}, cobra.ShellCompDirectiveNoFileComp)
	_ = root.RegisterFlagCompletionFunc("audience", audience)
	_ = root.MarkPersistentFlagFilename("config", "json")
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		if cmd.ValidArgsFunction == nil {
			cmd.ValidArgsFunction = cobra.NoFileCompletions
		}
		if cmd.Flags().Lookup("body-file") != nil {
			_ = cmd.MarkFlagFilename("body-file", "json")
		}
		for _, child := range cmd.Commands() {
			visit(child)
		}
	}
	visit(root)
}
