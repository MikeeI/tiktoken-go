# project-tiktoken-go-fork

Project references: @package.json

## 1. Project Contract

1.1 This is a Go library fork of `github.com/pkoukk/tiktoken-go`, not a setup-project single-binary CLI repo.

1.2 The root package is the public API contract for import path `github.com/pkoukk/tiktoken-go`. Do not move production code into `src/`, `cmd/`, or `internal/` unless the user explicitly requests a breaking layout migration.

1.3 Runtime scope is OpenAI-compatible `tiktoken` behavior in Go: BPE tokenization, model-to-encoding lookup, special-token handling, decoding, and `.tiktoken` BPE dictionary loading/caching.

1.4 Upstream parity beats local style cleanup. Tokenizer behavior changes require direct evidence from existing token expectations, focused parity checks, or an observed benchmark/smoke result.

## 2. Active Reads

2.1 MANDATORY: Before changing dependencies, module compatibility, or the public import contract, read `go.mod` and `go.sum` with the file-reading tool and use them as active context.

2.2 MANDATORY: Before changing build, test, lint, benchmark, or corpus workflows, read `Makefile` with the file-reading tool and use it as active context.

2.3 MANDATORY: Before performance work, locate the latest snapshot under `docs/benchmarks/` with the file-finding tool, read it with the file-reading tool, and use it as the current baseline.

2.4 MANDATORY: Before changing benchmark corpus inputs, read `testdata/bench/github_manifest.txt` and `tools/bench-corpus/main.go` with the file-reading tool. The manifest records source snapshots and hashes; do not infer corpus provenance from README prose.

## 3. Production Boundaries

3.1 Production library files live at repository root: `tiktoken.go`, `encoding.go`, `core_bpe.go`, `bpe.go`, and `load.go`.

3.2 Public entry points include `GetEncoding`, `EncodingForModel`, `SetBpeLoader`, `NewDefaultBpeLoader`, `NewCoreBPE`, `NewTiktoken`, `Encode`, `EncodeOrdinary`, and `Decode`.

3.3 `BpeLoader` is the extension point for offline/custom dictionary loading. Prefer it in tests/tools instead of adding alternate global loading paths.

3.4 Do not refactor `core_bpe.go` or `bpe.go` for aesthetics. Their control flow is tokenizer-parity-sensitive; simplify only with behavior-preserving evidence.

3.5 Model registry updates belong near model-to-encoding data and must cover exact model names and prefixes.

3.6 Loader/cache changes must preserve error behavior, cache atomicity, and failed-response non-caching.

## 4. Non-Production Boundaries

4.1 `testdata/` contains fixtures and benchmark corpora. Treat these as inputs, not production behavior.

4.2 `tools/token-num/` is a debug/comparison CLI, not public library API.

4.3 `tools/bench/`, `tools/bench-corpus/`, and `tools/legacy-python/` are benchmark/parity helpers. Keep them isolated from production package behavior.

4.4 `docs/benchmark-results.md` is historical comparison output only. Tests, observed runs, and current benchmark snapshots are stronger evidence.

## 5. Operations

5.1 Use `Makefile` as the operator surface: `make build`, `make test`, `make lint`, `make bench-corpus`, and `make bench`.

5.2 `make build` is compile-only package verification, `make test` runs race tests, and `make lint` runs fixer-first `golangci-lint`.

5.3 Do not add `VERSION`, `src/buildinfo/version.go`, `bin/<app>`, `/usr/local/bin` symlinks, or `/root/go/bin` symlinks for this library.

5.4 `make bench-corpus` builds `testdata/bench/github_corpus.txt` from one-time GitHub source snapshots under `testdata/bench/github/`. Existing snapshots must be reused; do not re-download unless the user explicitly asks to refresh the corpus.

5.5 `make bench` writes timestamped benchmark snapshots to `docs/benchmarks/`. Keep output compatible with Go benchmark tooling: timestamp, commit, command, Go version, raw benchmark rows, and elapsed wall time.

## 6. Benchmark Policy

6.1 Benchmark cases must use unique corpus slices, not repeated large strings.

6.2 Benchmark functions must not print debug output; use `b.N`, `b.ReportAllocs()`, `b.SetBytes()`, and explicit fixture paths.

6.3 Primary performance KPI is speed: lower `ns/op` and higher `MB/s` on representative corpus benchmarks. Allocation reductions matter only when they preserve or improve throughput, remove a measured bottleneck, or enable a verified follow-up speed win.

6.4 Commit benchmark snapshots only when intentionally establishing or updating a baseline. Do not auto-commit every local run.

6.5 Current benchmark snapshot is `docs/benchmarks/20260618T052119Z.txt`, generated at commit `9cdbec0` with Go `go1.26.4 linux/amd64` on `AMD Ryzen 9 5950X 16-Core Processor`.

6.6 Performance progress versus baseline `docs/benchmarks/20260618T041338Z.txt`:

| Case | ns/op median | Speed change | B/op median | B/op change | allocs/op median | allocs change | Read |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| `BenchmarkEncoding/fixture` | 56,768 | 4.4% faster | 17,120 | 23.4% lower | 231 | 17.5% lower | speed win |
| `BenchmarkEncoding/github/64KiB` | 1,757,628 | 91.8% faster | 772,968 | 91.4% lower | 1,694 | 98.3% lower | speed win |
| `BenchmarkEncoding/github/1MiB` | 27,294,986 | 90.4% faster | 13,170,684 | 90.5% lower | 33,794 | 97.6% lower | speed win |
| `BenchmarkEncoding/github/4MiB` | 70,703,264 | 92.7% faster | 48,360,208 | 90.0% lower | 82,323 | 98.3% lower | speed win |

6.7 New targeted benchmark surfaces:

| Case | Current median | Current allocation | Purpose |
| --- | ---: | ---: | --- |
| `BenchmarkEncodingForModelLookup` | 66.03 ns/op | 24 B/op, 1 alloc/op | Confirms encoding/core state reuse removes repeated cold initialization. |
| `BenchmarkEncoding/adversarial/long_ascii_single_piece` | 29,464,613 ns/op | 11,763,712 B/op, 5 allocs/op | Confirms heap BPE merge plus o200k ASCII scanning still removes the prior multi-second O(N²) worst case. |

6.8 Current read of progress: the o200k ordinary-encode fast route bypasses `regexp2` for ASCII spans and keeps small non-ASCII islands on the parity-safe regex path. Next performance work should target `encodeNative` with allowed specials and equivalent ASCII scanners for `cl100k_base`, `p50k_base`, and `r50k_base`.

## 7. Verification Policy

7.1 Keep tests deterministic by default. External network parity checks must be explicit, isolated, and not presented as unit-test evidence.

7.2 Add tests only for stable behavior contracts with material risk: tokenizer parity, model registry mapping, loader/cache error behavior, decode/encode invariants, or regressions that affect users.

7.3 For tooling, corpus, and benchmark workflow changes, prefer `make build`, `make test`, `make lint`, `make bench-corpus`, `make bench`, and targeted smoke runs over broad new tests.
