# project-tiktoken-go-fork

## 1. Repository Identity

1.1 This repository is a Go library fork of `github.com/pkoukk/tiktoken-go`.

1.2 The library ports OpenAI `tiktoken` behavior to Go: BPE tokenization, model-to-encoding lookup, special-token handling, decoding, and `.tiktoken` BPE dictionary loading/caching.

1.3 The public import path is `github.com/pkoukk/tiktoken-go`. Root package layout is part of the compatibility contract; do not move production library code into `src/`, `cmd/`, or `internal/` unless the user explicitly requests a breaking layout migration.

1.4 Upstream parity matters more than local style cleanup. Tokenizer algorithm changes require focused verification against existing token expectations or an explicit parity smoke check.

## 2. Production Surface

2.1 Runtime library files live at repository root: `tiktoken.go`, `encoding.go`, `core_bpe.go`, `bpe.go`, and `load.go`.

2.2 Public entry points include `GetEncoding`, `EncodingForModel`, `SetBpeLoader`, `NewDefaultBpeLoader`, `NewCoreBPE`, `NewTiktoken`, `Encode`, `EncodeOrdinary`, and `Decode`.

2.3 `go.mod` and `go.sum` are production dependency contracts. Do not change the minimum Go version or dependency versions as incidental cleanup; doing so changes consumer compatibility.

2.4 `BpeLoader` is the extension point for offline/custom dictionary loading. Prefer using it in tests/tools instead of adding alternate global loading paths.

## 3. Non-Production Assets

3.1 `testdata/` contains fixtures consumed by tests, benchmarks, and comparison tools.

3.2 `tools/token-num/` is a debug/comparison CLI, not public library API.

3.3 `tools/bench/` and `tools/legacy-python/` are benchmark/parity helpers. Keep them isolated from production package behavior.

3.4 `docs/benchmark-results.md` is historical comparison output. It is not a source of truth for tokenizer behavior; tests and observed runs are.

## 4. Repository Operations

4.1 `Makefile` is library-aware. `make build` performs compile-only package checks, `make test` runs race tests, and `make lint` runs fixer-first `golangci-lint`.

4.2 This is not a template-standard single-binary CLI repo. Do not add `VERSION`, `src/buildinfo/version.go`, `bin/<app>`, `/usr/local/bin` symlinks, or `/root/go/bin` symlinks for this library.

4.3 `make bench-corpus` builds `testdata/bench/github_corpus.txt` from one-time GitHub source snapshots under `testdata/bench/github/`. Existing snapshots must be reused and not fetched again.

4.4 `make bench` writes timestamped benchmark results to `docs/benchmarks/<UTC>.txt`. Keep the output compatible with Go benchmark tooling and include timestamp, commit, command, Go version, raw benchmark rows, and elapsed wall time.

4.5 Keep benchmarks deterministic: no debug prints in `Benchmark*`, use `b.N`, `b.ReportAllocs()`, explicit fixture paths, and unique corpus slices instead of repeated large strings.

4.6 Keep tests deterministic by default. External network parity checks must be explicit, isolated, and not presented as unit-test evidence.

## 5. Change Policy

5.1 Do not refactor `core_bpe.go` or `bpe.go` for aesthetics. The control flow mirrors tokenizer parity; simplify only with behavior-preserving evidence.

5.2 Model registry updates belong near model-to-encoding data and must include direct coverage for exact names and prefixes.

5.3 Loader/cache changes must preserve error behavior, cache atomicity, and failed-response non-caching.

## 6. Current Benchmark Status

6.1 Current baseline source is `docs/benchmarks/20260618T041338Z.txt`.

6.2 Benchmark environment: commit `6cf0e9b`, branch `main`, Go `go1.26.4 linux/amd64`, CPU `AMD Ryzen 9 5950X 16-Core Processor`.

6.3 Benchmark command: `go test -bench=BenchmarkEncoding -benchmem -benchtime=1s -count=5 -run '^$' .`.

6.4 Benchmark corpus: `testdata/bench/github_corpus.txt`, generated from 18 one-time GitHub source snapshots, total unique source bytes `5,147,273`.

6.5 Median baseline results from five runs:

| Case | ns/op median | MB/s median | B/op median | allocs/op median |
| --- | ---: | ---: | ---: | ---: |
| `BenchmarkEncoding/fixture` | 59,379 | 3.77 | 22,336 | 280 |
| `BenchmarkEncoding/github/64KiB` | 21,374,753 | 3.07 | 8,975,372 | 100,471 |
| `BenchmarkEncoding/github/1MiB` | 285,230,722 | 3.68 | 138,477,572 | 1,423,013 |
| `BenchmarkEncoding/github/4MiB` | 966,270,095 | 4.34 | 482,517,664 | 4,962,996 |

6.6 Current performance hotspot is allocation pressure in `Encode` on larger source corpus slices: `github/4MiB` allocates about `482MB/op` and about `4.96M allocs/op`.
