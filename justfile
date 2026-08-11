set shell := ["zsh", "-lc"]

default:
    @just --list

help:
    @just --list

list:
    bun run src/main.ts list

clean *mods:
    bun run src/main.ts clean {{mods}}

build *mods:
    bun run src/main.ts build {{mods}}

install *mods:
    bun run src/main.ts install {{mods}}
