# Audience is a prefix, not --output

`jenkins-cli` uses `cli [agent|human] <command>` and treats omitted Audience as `human`. This CLI copies that contract.

The prefix is what skills will copy. Swapping later means rewriting call sites. `agent` never means a Codeup user or a git author.
