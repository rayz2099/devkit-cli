export function fishCompletion(): string {
  return String.raw`complete -c gh-image-cli -f

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
`;
}
