function __mysql_cli_profiles
    mysql-cli __fish_complete_profiles
end

complete -c mysql-cli -f
complete -c mysql-cli -s p -x -a "(__mysql_cli_profiles)" -d "Profile"
complete -c mysql-cli -s e -x -d "Execute SQL and exit"
complete -c mysql-cli -l output -x -a "json csv" -d "Format -e result"
complete -c mysql-cli -a help -d "Show help"
