package cli

import (
	"fmt"
	"strings"
)

func IsCompletionRequest(args []string) bool {
	if len(args) != 2 {
		return false
	}
	return args[0] == "completion" && args[1] == "fish"
}

func CompletionScript(args []string) (string, error) {
	if !IsCompletionRequest(args) {
		return "", fmt.Errorf("usage: tl completion fish")
	}

	return strings.TrimSpace(`
complete -c tl -f
complete -c tl -l version -d 'Print current version'
complete -c tl -n '__fish_use_subcommand' -a en2zh -d 'Translate English plain text to Chinese'
complete -c tl -n '__fish_use_subcommand' -a zh2en -d 'Translate Chinese plain text to English'
complete -c tl -n '__fish_use_subcommand' -a md -d 'Translate Markdown'
complete -c tl -n '__fish_use_subcommand' -a help -d 'Show help'
complete -c tl -n '__fish_use_subcommand' -a completion -d 'Print shell completion script'

complete -c tl -n '__fish_seen_subcommand_from md; and not __fish_seen_subcommand_from en2zh zh2en' -a en2zh -d 'Translate English Markdown to Chinese'
complete -c tl -n '__fish_seen_subcommand_from md; and not __fish_seen_subcommand_from en2zh zh2en' -a zh2en -d 'Translate Chinese Markdown to English'
complete -c tl -n '__fish_seen_subcommand_from md' -l fast -d 'Pack Markdown units by document size'
complete -c tl -n '__fish_seen_subcommand_from md' -a "(__fish_complete_path)" -d 'Markdown file path'

complete -c tl -n '__fish_seen_subcommand_from completion; and not __fish_seen_subcommand_from fish' -a fish -d 'Fish shell completion'
`) + "\n", nil
}
