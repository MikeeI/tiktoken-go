# project-tiktoken-go-fork

## 1. Fork Setup Contract

1.1 This repository is a Go library fork of `github.com/pkoukk/tiktoken-go`, not a template-standard single-binary CLI repo.

1.2 Setup-project Go CLI files that assume `src/main.go`, `VERSION`, `src/buildinfo/version.go`, `bin/<app>`, `/usr/local/bin` symlinks, or `/root/go/bin` symlinks are intentionally not applied here.

1.3 `Makefile` is library-aware: `make build` runs compile-only package checks, `make test` runs race tests, and `make lint` runs fixer-first `golangci-lint`.

1.4 Keep public library package files at repository root unless a deliberate library-layout migration is explicitly requested. Do not move code into `src/` only to satisfy CLI-template defaults.

1.5 Upstream parity matters: tokenizer behavior changes require focused tests or smoke verification against existing token expectations.
