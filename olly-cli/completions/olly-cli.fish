complete -c olly-cli -f
complete -c olly-cli -s f -d 'Config file path' -r
complete -c olly-cli -s o -l output -d 'Output mode' -xa 'human agent plain'
complete -c olly-cli -s h -l help -d 'Show help'

complete -c olly-cli -n '__fish_seen_subcommand_from uptrace; and not __fish_seen_subcommand_from groups group-stats spans trace context diagnose' -a groups -d 'Query span groups with UQL'
complete -c olly-cli -n '__fish_seen_subcommand_from uptrace; and not __fish_seen_subcommand_from groups group-stats spans trace context diagnose' -a group-stats -d 'Query selected group metric columns'
complete -c olly-cli -n '__fish_seen_subcommand_from uptrace; and not __fish_seen_subcommand_from groups group-stats spans trace context diagnose' -a spans -d 'Query span list'
complete -c olly-cli -n '__fish_seen_subcommand_from uptrace; and not __fish_seen_subcommand_from groups group-stats spans trace context diagnose' -a trace -d 'Query a trace and print call tree'
complete -c olly-cli -n '__fish_seen_subcommand_from uptrace; and not __fish_seen_subcommand_from groups group-stats spans trace context diagnose' -a context -d 'Query a trace and print LLM context'
complete -c olly-cli -n '__fish_seen_subcommand_from uptrace; and not __fish_seen_subcommand_from groups group-stats spans trace context diagnose' -a diagnose -d 'Query service groups as diagnose entry'
complete -c olly-cli -n '__fish_seen_subcommand_from prometheus prom; and not __fish_seen_subcommand_from query ready healthy build-info runtime-info' -a query -d 'Prometheus query API'
complete -c olly-cli -n '__fish_seen_subcommand_from prometheus prom; and not __fish_seen_subcommand_from query ready healthy build-info runtime-info' -a ready -d 'Run /-/ready'
complete -c olly-cli -n '__fish_seen_subcommand_from prometheus prom; and not __fish_seen_subcommand_from query ready healthy build-info runtime-info' -a healthy -d 'Run /-/healthy'
complete -c olly-cli -n '__fish_seen_subcommand_from prometheus prom; and not __fish_seen_subcommand_from query ready healthy build-info runtime-info' -a build-info -d 'Run build info API'
complete -c olly-cli -n '__fish_seen_subcommand_from prometheus prom; and not __fish_seen_subcommand_from query ready healthy build-info runtime-info' -a runtime-info -d 'Run runtime info API'
complete -c olly-cli -n '__fish_seen_subcommand_from prometheus prom query; and not __fish_seen_subcommand_from instant range labels series' -a instant -d 'Run /api/v1/query'
complete -c olly-cli -n '__fish_seen_subcommand_from prometheus prom query; and not __fish_seen_subcommand_from instant range labels series' -a range -d 'Run /api/v1/query_range'
complete -c olly-cli -n '__fish_seen_subcommand_from prometheus prom query; and not __fish_seen_subcommand_from instant range labels series' -a labels -d 'Run labels API'
complete -c olly-cli -n '__fish_seen_subcommand_from prometheus prom query; and not __fish_seen_subcommand_from instant range labels series' -a series -d 'Run series API'
complete -c olly-cli -n 'not __fish_seen_subcommand_from uptrace prometheus prom logs graylog' -a uptrace -d 'Uptrace commands'
complete -c olly-cli -n 'not __fish_seen_subcommand_from uptrace prometheus prom logs graylog' -a prometheus -d 'Prometheus HTTP API'
complete -c olly-cli -n 'not __fish_seen_subcommand_from uptrace prometheus prom logs graylog' -a prom -d 'Alias for prometheus'
complete -c olly-cli -n 'not __fish_seen_subcommand_from uptrace prometheus prom logs graylog' -a logs -d 'Graylog logs search'
complete -c olly-cli -n 'not __fish_seen_subcommand_from uptrace prometheus prom logs graylog' -a graylog -d 'Alias for logs'
complete -c olly-cli -n '__fish_seen_subcommand_from logs graylog; and not __fish_seen_subcommand_from aggregate agg' -a aggregate -d 'Graylog server-side count aggregation'
complete -c olly-cli -n '__fish_seen_subcommand_from logs graylog; and not __fish_seen_subcommand_from aggregate agg' -a agg -d 'Alias for aggregate'

complete -c olly-cli -n '__fish_seen_subcommand_from groups group-stats diagnose context' -l service -d 'Service name' -r
complete -c olly-cli -n '__fish_seen_subcommand_from groups group-stats diagnose context' -l env -d 'Deployment environment' -r
complete -c olly-cli -n '__fish_seen_subcommand_from groups group-stats diagnose spans' -l query -d 'Raw Uptrace UQL' -r
complete -c olly-cli -n '__fish_seen_subcommand_from groups group-stats diagnose' -l search -d 'Span search expression' -r
complete -c olly-cli -n '__fish_seen_subcommand_from groups spans trace context diagnose' -l limit -d 'Result limit' -r
complete -c olly-cli -n '__fish_seen_subcommand_from groups spans trace context diagnose' -l time-gte -d 'Compact start time, e.g. 20260429T060000' -r
complete -c olly-cli -n '__fish_seen_subcommand_from groups spans trace context diagnose' -l time-dur -d 'Time duration seconds' -r
complete -c olly-cli -n '__fish_seen_subcommand_from groups spans trace context diagnose' -l time-start -d 'RFC3339 start time' -r
complete -c olly-cli -n '__fish_seen_subcommand_from groups spans trace context diagnose' -l time-end -d 'RFC3339 end time' -r
complete -c olly-cli -n '__fish_seen_subcommand_from groups spans trace context diagnose' -l time-lt -d 'Internal API end time' -r
complete -c olly-cli -n '__fish_seen_subcommand_from spans trace context' -l trace-id -d 'Trace id' -r
complete -c olly-cli -n '__fish_seen_subcommand_from spans' -l span-id -d 'Span id' -r
complete -c olly-cli -n '__fish_seen_subcommand_from spans' -l parent-id -d 'Parent span id' -r
complete -c olly-cli -n '__fish_seen_subcommand_from spans' -l sort-by -d 'Sort column, e.g. _duration' -r
complete -c olly-cli -n '__fish_seen_subcommand_from spans' -l sort-desc -d 'Sort descending'
complete -c olly-cli -n '__fish_seen_subcommand_from spans' -l page -d 'Page number' -r
complete -c olly-cli -n '__fish_seen_subcommand_from groups spans trace context diagnose' -l duration-gte -d 'Min duration in microseconds' -r
complete -c olly-cli -n '__fish_seen_subcommand_from groups spans trace context diagnose' -l duration-lt -d 'Max duration in microseconds' -r
complete -c olly-cli -n '__fish_seen_subcommand_from context' -l uri -d 'HTTP route or uri hint' -r
complete -c olly-cli -n '__fish_seen_subcommand_from prometheus prom' -l base-url -d 'Prometheus base URL' -r
complete -c olly-cli -n '__fish_seen_subcommand_from instant range' -l timeout -d 'Prometheus query timeout' -r
complete -c olly-cli -n '__fish_seen_subcommand_from instant range labels series' -l limit -d 'Prometheus result limit' -r
complete -c olly-cli -n '__fish_seen_subcommand_from instant' -l time -d 'Evaluation time' -r
complete -c olly-cli -n '__fish_seen_subcommand_from range labels series' -l start -d 'Start time' -r
complete -c olly-cli -n '__fish_seen_subcommand_from range labels series' -l end -d 'End time' -r
complete -c olly-cli -n '__fish_seen_subcommand_from range' -l step -d 'Range query step' -r
complete -c olly-cli -n '__fish_seen_subcommand_from labels series' -l match -d 'Series selector' -r
complete -c olly-cli -n '__fish_seen_subcommand_from range instant' -l include-values -d 'Include raw values in agent output'
complete -c olly-cli -n '__fish_seen_subcommand_from logs graylog' -l query -d 'Graylog query' -r
complete -c olly-cli -n '__fish_seen_subcommand_from logs graylog' -l relative -d 'Relative time range seconds' -r
complete -c olly-cli -n '__fish_seen_subcommand_from logs graylog' -l from -d 'Absolute start time' -r
complete -c olly-cli -n '__fish_seen_subcommand_from logs graylog' -l to -d 'Absolute end time' -r
complete -c olly-cli -n '__fish_seen_subcommand_from logs graylog' -l keyword -d 'Keyword time range' -r
complete -c olly-cli -n '__fish_seen_subcommand_from logs graylog' -l limit -d 'Result limit' -r
complete -c olly-cli -n '__fish_seen_subcommand_from logs graylog' -l offset -d 'Result offset' -r
complete -c olly-cli -n '__fish_seen_subcommand_from logs graylog' -l sort -d 'Sort expression' -r
complete -c olly-cli -n '__fish_seen_subcommand_from logs graylog' -l fields -d 'Returned fields' -r
complete -c olly-cli -n '__fish_seen_subcommand_from logs graylog' -l filter -d 'Graylog filter' -r
complete -c olly-cli -n '__fish_seen_subcommand_from logs graylog' -l decorate -d 'Decorate messages'
complete -c olly-cli -n '__fish_seen_subcommand_from logs graylog' -l group-by -d 'Group by message field' -r
complete -c olly-cli -n '__fish_seen_subcommand_from logs graylog' -l show-fields -d 'Print fields available in this query result'
complete -c olly-cli -n '__fish_seen_subcommand_from aggregate agg' -l field -d 'Aggregate field' -r
