export const HELP_TEXT = `olly-cli

Usage:
  olly-cli [-f config.json] [--output human|agent|plain] uptrace <command> [options]
  olly-cli [-f config.json] [--output human|agent|plain] logs <query-or-graylog-url> [options]
  olly-cli [-f config.json] [--output human|agent|plain] prometheus <command> [options]
  olly-cli [-f config.json] [--output human|agent|plain] prom <command> [options]
  olly-cli --help

Global options:
  -f <path>                 Config file path. Default: ~/.config/olly-cli/config.json
  -o, --output <mode>       Output mode: human, agent, plain. Default: human
  -h, --help                Show help

Uptrace commands:
  uptrace groups            Query span groups with UQL
  uptrace group-stats       Query selected group metric columns
  uptrace spans             Query span list
  uptrace trace             Query a trace and print call tree
  uptrace context           Query a trace and print LLM context
  uptrace diagnose          Query service groups as the first diagnose step

Prometheus commands:
  prometheus query instant  Run /api/v1/query
  prometheus query range    Run /api/v1/query_range
  prometheus query labels   Run /api/v1/labels or label values API
  prometheus query series   Run /api/v1/series
  prometheus ready          Run /-/ready
  prometheus healthy        Run /-/healthy
  prometheus build-info     Run /api/v1/status/buildinfo
  prometheus runtime-info   Run /api/v1/status/runtimeinfo
  prom                      Alias for prometheus

Graylog commands:
  logs                     Query Graylog relative search API
  graylog                  Alias for logs

Uptrace internal API:
  groups: /internal/v1/tracing/{project_id}/groups
  stats:  /internal/v1/tracing/{project_id}/group-stats
  spans:  /internal/v1/tracing/{project_id}/spans
  trace:  /internal/v1/tracing/{project_id}/traces/{trace_id}

Common examples:
  olly-cli uptrace groups --service app-gw --env loadtest --time-gte 20260429T060000 --time-dur 10800
  olly-cli uptrace spans --query 'where service_name = "ld-dt-gateway" | where _system = "httpserver:ld-dt-gateway"' --sort-by _duration --sort-desc --limit 15
  olly-cli uptrace trace https://uptrace.example.com/traces/2/e4f1e0bcd6a1ac296661a3e6ea5507c9
  olly-cli --output agent uptrace context e4f1e0bcd6a1ac296661a3e6ea5507c9 --service app-gw --env loadtest
  olly-cli --output agent logs 'app:billing AND level:3' --relative 28800 --group-by logger_name
  olly-cli logs 'https://log.example.com/search?q=app%3Abilling+AND+level%3A3&rangetype=relative&relative=28800'
  olly-cli prometheus query labels __name__
  olly-cli prometheus query series --match='up' | jq -r '.[].job'
  olly-cli prometheus query range --start='2026-04-22T10:00:00+08:00' --end='2026-04-22T11:00:00+08:00' --step=60s 'up'

Local install:
  just install
  just fish-setup

Config:
  {
    "uptrace": {
      "base_url": "https://uptrace.example.com",
      "web_base_url": "https://uptrace.example.com",
      "project_id": 2,
      "auth_token": "user-auth-token",
      "jwt_token": "browser-jwt-token",
      "default_env": "loadtest",
      "default_time_dur_seconds": 10800
    },
    "prometheus": {
      "base_url": "http://127.0.0.1:9090",
      "default_step": "60s",
      "default_timeout": "30s"
    },
    "graylog": {
      "base_url": "127.0.0.1:9000",
      "username": "admin",
      "password": "admin"
    }
  }

Auth:
  Use either auth_token for Bearer auth or jwt_token for Cookie auth.
  When jwt_token is configured, requests use: Cookie: token=<jwt_token>

Prometheus:
  Default endpoint: http://127.0.0.1:9090
  Prometheus commands use the HTTP API directly and do not require promtool.

Graylog:
  logs output groups messages by logger_name and preserves trace_id for Uptrace follow-up.
`;

export const UPTRACE_HELP_TEXT = `olly-cli uptrace

Usage:
  olly-cli [-f config.json] [--output human|agent|plain] uptrace <command> [options]
  olly-cli uptrace --help

Commands:
  groups            Query span groups with UQL
  group-stats       Query selected group metric columns
  spans             Query span list
  trace             Query a trace and print call tree
  context           Query a trace and print LLM context
  diagnose          Query service groups as the first diagnose step

Common options:
  --service <name>          Service name
  --env <env>               Deployment environment
  --query <uql>             Raw Uptrace UQL
  --limit <n>               Result limit
  --time-gte <time>         Compact start time, e.g. 20260429T060000
  --time-dur <seconds>      Time duration seconds
  --trace-id <id>           Trace id

Examples:
  olly-cli uptrace groups --service app-gw --env loadtest --time-gte 20260429T060000 --time-dur 10800
  olly-cli uptrace spans --query 'where service_name = "ld-dt-gateway"' --sort-by _duration --sort-desc --limit 15
  olly-cli uptrace trace https://uptrace.example.com/traces/2/e4f1e0bcd6a1ac296661a3e6ea5507c9
  olly-cli --output agent uptrace context e4f1e0bcd6a1ac296661a3e6ea5507c9 --service app-gw --env loadtest
`;

export const PROMETHEUS_HELP_TEXT = `olly-cli prometheus

Usage:
  olly-cli [-f config.json] [--output human|agent|plain] prometheus <command> [options]
  olly-cli [-f config.json] [--output human|agent|plain] prom <command> [options]
  olly-cli prometheus --help

Commands:
  query instant      Run /api/v1/query
  query range        Run /api/v1/query_range
  query labels       Run /api/v1/labels or label values API
  query series       Run /api/v1/series
  ready              Run /-/ready
  healthy            Run /-/healthy
  build-info         Run /api/v1/status/buildinfo
  runtime-info       Run /api/v1/status/runtimeinfo

Options:
  --base-url <url>          Prometheus base URL. Default: http://127.0.0.1:9090
  --start <time>            Range start time
  --end <time>              Range end time
  --step <duration>         Range query step
  --time <time>             Instant query evaluation time
  --timeout <duration>      Prometheus query timeout
  --limit <n>               Result limit
  --match <selector>        Series selector, repeatable
  --include-values          Include raw values in agent output

Examples:
  olly-cli prometheus query instant 'up{job="prometheus"}'
  olly-cli prometheus query labels __name__
  olly-cli prometheus query labels job --match='up'
  olly-cli prometheus query series --match='up'
  olly-cli prometheus query range --start='2026-04-22T10:00:00+08:00' --end='2026-04-22T11:00:00+08:00' --step=60s 'up'
`;

export const GRAYLOG_HELP_TEXT = `olly-cli logs

Usage:
  olly-cli [-f config.json] [--output human|agent|plain] logs <query-or-graylog-url> [options]
  olly-cli [-f config.json] [--output human|agent|plain] logs aggregate --field <field> --query <query> [options]
  olly-cli [-f config.json] [--output human|agent|plain] graylog <query-or-graylog-url> [options]
  olly-cli logs --help

Options:
  --query <query>           Graylog query, e.g. app:billing AND level:3
  --relative <seconds>      Relative time range seconds. Default: 28800
  --from <time>             Absolute start time
  --to <time>               Absolute end time
  --keyword <range>         Keyword range, e.g. last 5 minutes
  --limit <n>               Result limit. Default: 50
  --offset <n>              Result offset
  --sort <field:dir>        Sort expression. Default: timestamp:desc
  --fields <fields>         Comma-separated returned fields
  --filter <filter>         Graylog filter, e.g. streams:<id>
  --decorate                Ask Graylog to decorate messages
  --group-by <field>        Group returned messages by any message field
  --show-fields             Print fields available in this query result
  --field <field>           Aggregate field for logs aggregate

Examples:
  olly-cli logs 'app:billing AND level:3' --relative 28800 --limit 20
  olly-cli logs --query 'app:billing AND level:3' --show-fields
  olly-cli logs aggregate --query 'app:billing AND level:3' --relative 28800 --field USER_IP --limit 10
  olly-cli --output agent logs 'app:billing AND level:3' --group-by logger_name
  olly-cli logs 'app:billing AND level:3' --from '2026-06-01T10:00:00+08:00' --to '2026-06-01T11:00:00+08:00'
  olly-cli logs 'https://log.example.com/search?q=app%3Abilling+AND+level%3A3&rangetype=relative&relative=28800'
`;

/** 为什么：help 必须在读取配置前可用，方便首次安装后自检。 */
export function shouldShowHelp(argv: string[]): boolean {
  if (argv.length === 0 || isHelpToken(argv[0])) {
    return true;
  }
  return isRootCommand(argv[0]) && isHelpToken(argv[1]);
}

export function helpTextFor(argv: string[]): string {
  if (argv[0] === "uptrace") {
    return UPTRACE_HELP_TEXT;
  }
  if (argv[0] === "prometheus" || argv[0] === "prom") {
    return PROMETHEUS_HELP_TEXT;
  }
  if (argv[0] === "logs" || argv[0] === "graylog") {
    return GRAYLOG_HELP_TEXT;
  }
  return HELP_TEXT;
}

function isHelpToken(value: string | undefined): boolean {
  return value === "--help" || value === "-h" || value === "help";
}

function isRootCommand(value: string | undefined): boolean {
  return value === "uptrace" || value === "prometheus" || value === "prom" || value === "logs" || value === "graylog";
}
