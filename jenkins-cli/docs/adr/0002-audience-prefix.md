# Audience is an optional argv prefix

`olly-cli` uses `--output human|agent`. This CLI uses `jenkins-cli [agent] <command>` and treats omitted Audience as `human`.

The prefix is the public contract skills will copy. Swapping later means rewriting `$dt-deploy` / `$tt-verify` call sites. `agent` never means a Jenkins node; that concept is `node` and is out of v1.
