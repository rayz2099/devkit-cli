package cli

import "strings"

func IsHelpRequest(args []string) bool {
	if len(args) == 0 {
		return true
	}

	normalized := make([]string, 0, len(args))
	for _, arg := range args {
		normalized = append(normalized, strings.TrimSpace(arg))
	}

	switch normalized[0] {
	case "-h", "--help", "help":
		return true
	case "md":
		if len(normalized) < 2 {
			return true
		}
		switch normalized[1] {
		case "-h", "--help", "help":
			return true
		}
	}
	return false
}

func HelpText(args []string) string {
	if len(args) > 0 && args[0] == "md" {
		return strings.TrimSpace(`
Usage:
  tl md en2zh [file]
  tl md zh2en [file]
  tl md en2zh --fast [file]

Flags:
  --fast      pack text units by document size into fewer LLM requests

Behavior:
  - [file] omitted: read Markdown from stdin
  - translated Markdown is written to stdout
  - progress bar and errors are written to stderr
  - translations run per text unit; no batch requests
  - --fast keeps AST extract/splice and only changes packing

Config:
  ~/.config/tl/config.json
  providers.openai.base_url
  providers.openai.token
  providers.openai.model
  providers.openai.concurrency (optional, default 8)
  providers.openai.custom_prompt
`) + "\n"
	}

	return strings.TrimSpace(`
Usage:
  tl en2zh [text]
  tl zh2en [text]
  tl md en2zh [file]
  tl md zh2en [file]
  tl md en2zh --fast [file]
  tl --version
  tl completion fish
  tl help

Commands:
  en2zh      Translate plain text from English to Chinese
  zh2en      Translate plain text from Chinese to English
  md      Translate Markdown via goldmark AST
  --version      Print current version
  completion      Print shell completion script

Examples:
  tl en2zh hello
  echo '你好' | tl zh2en
  tl md en2zh README.md
  cat README.md | tl md zh2en
  tl --version
  tl completion fish

Behavior:
  - [text] omitted: read plain text from stdin
  - [text] accepts a single argument only

Config:
  ~/.config/tl/config.json
  providers.openai.base_url
  providers.openai.token
  providers.openai.model
  providers.openai.concurrency (optional, default 8)
  providers.openai.custom_prompt
`) + "\n"
}
