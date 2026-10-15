package ghimage

// fishScript 是 fish 补全定义。别改文案：shell 里直接 source 这段输出。
const fishScript = `complete -c gh-image-cli -f
complete -c gh-image-cli -s c -l config -r -d 'config file'

function __gh_image_cli_aliases
    gh-image-cli __complete aliases 2>/dev/null
end

function __gh_image_cli_versions
    set -l tokens (commandline -opc)
    if test (count $tokens) -ge 3
        gh-image-cli __complete versions $tokens[3] 2>/dev/null
    end
end

complete -c gh-image-cli -n 'not __fish_seen_subcommand_from add add-dockerfile build list completion' -a add -d 'Register a mirrored image'
complete -c gh-image-cli -n 'not __fish_seen_subcommand_from add add-dockerfile build list completion' -a add-dockerfile -d 'Register a Dockerfile build script'
complete -c gh-image-cli -n 'not __fish_seen_subcommand_from add add-dockerfile build list completion' -a build -d 'Build or sync an image through GitHub Actions'
complete -c gh-image-cli -n 'not __fish_seen_subcommand_from add add-dockerfile build list completion' -a list -d 'List image aliases'
complete -c gh-image-cli -n 'not __fish_seen_subcommand_from add add-dockerfile build list completion' -a completion -d 'Generate shell completion'
complete -c gh-image-cli -n '__fish_seen_subcommand_from build list; and test (count (commandline -opc)) -eq 2' -a '(__gh_image_cli_aliases)'
complete -c gh-image-cli -n '__fish_seen_subcommand_from build; and test (count (commandline -opc)) -eq 3' -a '(__gh_image_cli_versions)'
complete -c gh-image-cli -n '__fish_seen_subcommand_from completion' -a fish
`

func (a *app) printFish() {
	// 模板自身已有结尾换行，再补一行，对齐原来的 console.log。
	fmtPrintln(a.out, fishScript)
}

func (a *app) complete(args []string) {
	kind := ""
	alias := ""
	if len(args) > 0 {
		kind = args[0]
	}
	if len(args) > 1 {
		alias = args[1]
	}
	local, err := resolveLocal(a.rt)
	if err != nil {
		return
	}
	book, err := loadStore(local.imagesPath)
	if err != nil {
		return
	}
	switch kind {
	case "aliases":
		fmtPrintln(a.out, joinLines(book.aliases()))
	case "versions":
		if alias == "" {
			return
		}
		vers, err := book.versions(alias)
		if err != nil {
			return
		}
		fmtPrintln(a.out, joinLines(vers))
	}
}
