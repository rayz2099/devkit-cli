package dsn

func helpText(topic string) string {
	switch topic {
	case "query":
		return `dsn-cli query

Usage:
  dsn-cli -p <profile> query '<stmt>' [--output json|csv|plain] [--pretty] [--limit N] [--timeout S]
  dsn-cli -p <profile> query peek <topic> [n]
  dsn-cli agent -p <profile> query '<stmt>' [--limit N] [--timeout S]
`
	case "doctor":
		return `dsn-cli doctor

Usage:
  dsn-cli doctor [-p <profile>] [--output json|csv|plain] [--pretty] [--timeout S] [--connect-timeout S]
  dsn-cli agent doctor [-p <profile>] [--timeout S]

Checks that the config file parses and each profile has a name, kind, and url. No network.
`
	case "ds":
		return `dsn-cli ds

Usage:
  dsn-cli ds [-p <profile>] [--output json|csv|plain] [--pretty]
  dsn-cli agent ds [-p <profile>]
  dsn-cli -h ds
  dsn-cli -p <profile> -h ds

Lists Profiles without connecting. Columns: name, kind, access, description. Url is never printed.
`
	default:
		return `dsn-cli

Usage:
  dsn-cli [-c config.json] -p <profile>
  dsn-cli [-c config.json] -p <profile> query '<stmt>' [--output json|csv|plain] [--pretty] [--limit N] [--timeout S]
  dsn-cli [-c config.json] -p <profile> query peek <topic> [n]
  dsn-cli [-c config.json] agent -p <profile> query '<stmt>' [--limit N] [--timeout S]
  dsn-cli [-c config.json] doctor [-p <profile>] [--timeout S] [--connect-timeout S]
  dsn-cli [-c config.json] agent doctor [-p <profile>] [--timeout S]
  dsn-cli [-c config.json] ds [-p <profile>] [--output json|csv|plain] [--pretty]
  dsn-cli [-c config.json] agent ds [-p <profile>]
  dsn-cli [-c config.json] -h ds
  dsn-cli completion fish
  dsn-cli version

TTY -p opens the vendor client (mysql / psql / redis-cli / mongosh).
agent and non-TTY cannot enter that Console.
doctor only checks the config file; it does not connect.
`
	}
}

func fishScript() string {
	return `function __dsn_cli_complete
    set -l tokens (commandline -opc)
    set -e tokens[1]
    set -l results (dsn-cli __complete $tokens (commandline -ct))
    if test (count $results) -eq 0
        return
    end
    printf '%s\n' $results
end

complete -c dsn-cli -e
complete -c dsn-cli -f
complete -c dsn-cli -a '(__dsn_cli_complete)'
complete -c dsn-cli -s c -l config -r -d 'config file'
complete -c dsn-cli -s p -l profile -x -a '(__dsn_cli_complete)' -d 'profile name'
complete -c dsn-cli -l output -x -a 'json csv plain' -d 'human query format'
complete -c dsn-cli -l pretty -d 'pretty-print json output'
complete -c dsn-cli -l limit -r -d 'agent row cap'
complete -c dsn-cli -l timeout -r -d 'query timeout seconds'
complete -c dsn-cli -l connect-timeout -r -d 'connect timeout seconds'
complete -c dsn-cli -s h -l help -d 'show help'
`
}
