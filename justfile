set shell := ["zsh", "-lc"]

default:
    @just --list

help:
    @just --list

list:
    mise exec -- go run ./cmd/devkit-cli list

clean *mods:
    mise exec -- go run ./cmd/devkit-cli clean {{mods}}

build *mods:
    mise exec -- go run ./cmd/devkit-cli build {{mods}}

install *mods:
    mise exec -- go run ./cmd/devkit-cli install {{mods}}
