package nacos

func helpText(topic string) string {
	var body string
	switch topic {
	case "config":
		body = `nacos-cli config

Usage:
  nacos-cli config get [data-id] [group]
  nacos-cli config put --data-id <id> --group <group> --content <text>
  nacos-cli config delete --data-id <id> --group <group>
  nacos-cli config list [--search blur|accurate] [--data-id <id>] [--group <group>]
`
	case "naming":
		body = `nacos-cli naming

Usage:
  nacos-cli naming register --service <name> --ip <ip> --port <port>
  nacos-cli naming deregister --service <name> --ip <ip> --port <port>
  nacos-cli naming instances --service <name> [--clusters a,b] [--healthy-only]
`
	default:
		body = `nacos-cli

Usage:
  nacos-cli [-c config.json] [global] config <get|put|delete|list>
  nacos-cli [-c config.json] [global] naming <register|deregister|instances>
  nacos-cli completion fish

Global:
  -c, --config <path>    config file path
  --server-addr <addr>   nacos server address
  --username <name>      nacos username
  --password <pass>      nacos password
  -ns, --namespace <id>  nacos namespace
  -o, --output text|json output format
  --dev                  write HTTP debug log
`
	}
	return body + "\n"
}
