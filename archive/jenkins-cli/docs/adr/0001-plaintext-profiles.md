# Plaintext Profile secrets

Old `jk` kept tokens in the OS keychain plus YAML. The replacement stores `password` and `apiToken` in `~/.config/jenkins-cli/config.json` in plaintext, matching `mysql-cli` / `nacos-cli`.

This is reversible only after every skill and local script stops assuming that path. Future readers will expect a keychain because the archived repo did that on purpose.

## Considered Options

- Keep keychain / encrypted file: rejected. This is a personal XDG tool; the user will fill the file by hand.
- Read `~/.config/skills/.env` `JENKINS_*`: rejected as the source of truth. That stays the old `jenkins.ts` contract until cutover.
