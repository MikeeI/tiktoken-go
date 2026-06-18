MODULE := $(shell go list -m)
Q      := $(if $(VERBOSE),,@)
BENCH_DIR ?= docs/benchmarks
BENCH_FILE ?= $(BENCH_DIR)/$(shell date -u +%Y%m%dT%H%M%SZ).txt
BENCH_RUN ?= BenchmarkEncoding
BENCH_TIME ?= 1s
BENCH_COUNT ?= 5

.PHONY: build test test-unit test-race lint lint-changed test-changed bench bench-corpus doctor doctor-build format generate clean uninstall tidy

define CHANGED_GO_PKGS
files="$$(git diff --name-only --diff-filter=ACMR HEAD -- '*.go'; git ls-files --others --exclude-standard -- '*.go')"; \
files="$$(printf '%s\n' "$$files" | sed '/^$$/d' | sort -u)"; \
if [ -z "$$files" ]; then \
	echo "No changed Go files."; \
	exit 0; \
fi; \
pkgs="$$(printf '%s\n' "$$files" | xargs -n1 dirname | sort -u | awk '{ if ($$0 == ".") print "./"; else print "./" $$0 }')"
endef

build:
	$(Q)output=$$(go test -run '^$$' ./... 2>&1); rc=$$?; \
	if [ $$rc -eq 0 ]; then echo "build: ok"; else echo "$$output"; fi; exit $$rc

test: test-unit

test-unit:
	$(Q)output=$$(gotestsum --format=pkgname-and-test-fails --format-hide-empty-pkg -- -race -count=1 ./... 2>&1); rc=$$?; \
	if [ $$rc -eq 0 ]; then echo "$$output" | tail -1; else echo "$$output"; fi; exit $$rc

test-race: test-unit

lint: tidy
	$(Q)output=$$(golangci-lint fmt 2>&1); rc=$$?; \
	if [ $$rc -ne 0 ]; then echo "$$output"; exit $$rc; fi
	$(Q)output=$$(golangci-lint run --fix ./... 2>&1); rc=$$?; \
	if [ $$rc -ne 0 ]; then echo "$$output"; exit $$rc; fi
	$(Q)output=$$(golangci-lint run ./... 2>&1); rc=$$?; \
	if [ $$rc -eq 0 ]; then echo "lint: ok"; else echo "$$output"; fi; exit $$rc

lint-changed:
	$(Q)$(CHANGED_GO_PKGS); \
	output=$$(golangci-lint fmt $$pkgs 2>&1); rc=$$?; \
	if [ $$rc -ne 0 ]; then echo "$$output"; exit $$rc; fi; \
	output=$$(golangci-lint run --fix $$pkgs 2>&1); rc=$$?; \
	if [ $$rc -ne 0 ]; then echo "$$output"; exit $$rc; fi; \
	output=$$(golangci-lint run $$pkgs 2>&1); rc=$$?; \
	if [ $$rc -eq 0 ]; then echo "lint-changed: ok"; else echo "$$output"; fi; exit $$rc

test-changed:
	$(Q)$(CHANGED_GO_PKGS); \
	gotestsum --format=pkgname-and-test-fails --format-hide-empty-pkg -- -race -count=1 $$pkgs

bench: bench-corpus
	$(Q)mkdir -p "$(BENCH_DIR)"; \
	start=$$(date -u +%s); \
	{ \
		echo "# tiktoken-go benchmark"; \
		echo "timestamp_utc: $$(date -u +%Y-%m-%dT%H:%M:%SZ)"; \
		echo "commit: $$(git rev-parse --short HEAD)"; \
		echo "branch: $$(git branch --show-current)"; \
		echo "go: $$(go version)"; \
		echo "command: go test -bench=$(BENCH_RUN) -benchmem -benchtime=$(BENCH_TIME) -count=$(BENCH_COUNT) -run '^$$' ."; \
		echo; \
		go test -bench="$(BENCH_RUN)" -benchmem -benchtime="$(BENCH_TIME)" -count="$(BENCH_COUNT)" -run '^$$' .; \
		rc=$$?; \
		end=$$(date -u +%s); \
		echo; \
		echo "elapsed_seconds: $$((end - start))"; \
	} > "$(BENCH_FILE)"; \
	if [ $$rc -eq 0 ]; then echo "bench: $(BENCH_FILE)"; else echo "bench failed: $(BENCH_FILE)"; exit $$rc; fi

bench-corpus:
	$(Q)go run ./tools/bench-corpus

doctor:
	$(Q)missing=0; \
	for tool in go goimports gofumpt golangci-lint gotestsum git node lint-staged husky; do \
		if ! command -v $$tool >/dev/null 2>&1; then \
			echo "missing tool: $$tool"; \
			missing=1; \
		fi; \
	done; \
	for path in go.mod .golangci.yml .lintstagedrc.js package.json Makefile; do \
		if [ ! -e "$$path" ]; then \
			echo "missing file: $$path"; \
			missing=1; \
		fi; \
	done; \
	if [ $$missing -eq 0 ]; then echo "doctor: ok"; else exit 1; fi

doctor-build: build

format: lint
	$(Q)echo "format: ok"

tidy:
	$(Q)output=$$(mktemp); go mod tidy >$$output 2>&1; rc=$$?; grep -v '^go: downloading' $$output; rm -f $$output; exit $$rc

generate:
	$(Q)go generate ./...

uninstall:
	$(Q)echo "uninstall: ok (library has no global binary)"

clean:
	rm -rf bin coverage.out
