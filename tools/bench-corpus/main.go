package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	benchCorpusDir    = "testdata/bench"
	sourceSnapshotDir = "testdata/bench/github"
	corpusPath        = "testdata/bench/github_corpus.txt"
	manifestPath      = "testdata/bench/github_manifest.txt"
	requestTimeout    = 60 * time.Second
	userAgent         = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"
)

type sourceFile struct {
	Name string
	URL  string
	File string
}

var sources = []sourceFile{
	{Name: "golang-net-http-server", URL: "https://raw.githubusercontent.com/golang/go/master/src/net/http/server.go", File: "01-golang-net-http-server.go"},
	{Name: "golang-runtime-proc", URL: "https://raw.githubusercontent.com/golang/go/master/src/runtime/proc.go", File: "02-golang-runtime-proc.go"},
	{Name: "golang-fmt-print", URL: "https://raw.githubusercontent.com/golang/go/master/src/fmt/print.go", File: "03-golang-fmt-print.go"},
	{Name: "kubernetes-kubelet", URL: "https://raw.githubusercontent.com/kubernetes/kubernetes/master/pkg/kubelet/kubelet.go", File: "04-kubernetes-kubelet.go"},
	{Name: "kubernetes-scheduler", URL: "https://raw.githubusercontent.com/kubernetes/kubernetes/master/pkg/scheduler/schedule_one.go", File: "05-kubernetes-scheduler.go"},
	{Name: "kubernetes-deployment-controller", URL: "https://raw.githubusercontent.com/kubernetes/kubernetes/master/pkg/controller/deployment/deployment_controller.go", File: "06-kubernetes-deployment-controller.go"},
	{Name: "typescript-checker", URL: "https://raw.githubusercontent.com/microsoft/TypeScript/main/src/compiler/checker.ts", File: "07-typescript-checker.ts"},
	{Name: "typescript-parser", URL: "https://raw.githubusercontent.com/microsoft/TypeScript/main/src/compiler/parser.ts", File: "08-typescript-parser.ts"},
	{Name: "vscode-text-model", URL: "https://raw.githubusercontent.com/microsoft/vscode/main/src/vs/editor/common/model/textModel.ts", File: "09-vscode-text-model.ts"},
	{Name: "vscode-workbench", URL: "https://raw.githubusercontent.com/microsoft/vscode/main/src/vs/workbench/browser/workbench.ts", File: "10-vscode-workbench.ts"},
	{Name: "react-fiber-work-loop", URL: "https://raw.githubusercontent.com/facebook/react/main/packages/react-reconciler/src/ReactFiberWorkLoop.js", File: "11-react-fiber-work-loop.js"},
	{Name: "react-dom-events", URL: "https://raw.githubusercontent.com/facebook/react/main/packages/react-dom-bindings/src/events/DOMPluginEventSystem.js", File: "12-react-dom-events.js"},
	{Name: "rust-hir", URL: "https://raw.githubusercontent.com/rust-lang/rust/master/compiler/rustc_hir/src/hir.rs", File: "13-rust-hir.rs"},
	{Name: "rust-ty-context", URL: "https://raw.githubusercontent.com/rust-lang/rust/master/compiler/rustc_middle/src/ty/context.rs", File: "14-rust-ty-context.rs"},
	{Name: "cpython-asyncio-base-events", URL: "https://raw.githubusercontent.com/python/cpython/main/Lib/asyncio/base_events.py", File: "15-cpython-asyncio-base-events.py"},
	{Name: "cpython-http-server", URL: "https://raw.githubusercontent.com/python/cpython/main/Lib/http/server.py", File: "16-cpython-http-server.py"},
	{Name: "node-cjs-loader", URL: "https://raw.githubusercontent.com/nodejs/node/main/lib/internal/modules/cjs/loader.js", File: "17-node-cjs-loader.js"},
	{Name: "node-http", URL: "https://raw.githubusercontent.com/nodejs/node/main/lib/http.js", File: "18-node-http.js"},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "bench corpus failed: %v\n", err)
		os.Exit(1)
	}
}

func run() (err error) {
	if err := os.MkdirAll(sourceSnapshotDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(benchCorpusDir, 0o755); err != nil {
		return err
	}

	corpus, err := os.Create(corpusPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, corpus.Close()) }()

	ctx := context.Background()
	client := &http.Client{Timeout: requestTimeout}
	manifest := strings.Builder{}
	manifest.WriteString("# GitHub benchmark corpus manifest\n")
	manifest.WriteString("# Source snapshots are fetched once and reused from testdata/bench/github/.\n")
	manifest.WriteString("# name\tbytes\tsha256\tfile\turl\n")

	totalBytes := 0
	for _, source := range sources {
		data, err := readOrDownload(ctx, client, source)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(data)
		sha := hex.EncodeToString(digest[:])
		totalBytes += len(data)

		if _, err := fmt.Fprintf(corpus, "\n\n===== %s =====\n%s\n\n", source.Name, source.URL); err != nil {
			return err
		}
		if _, err := corpus.Write(data); err != nil {
			return err
		}
		if _, err := corpus.WriteString("\n"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(&manifest, "%s\t%d\t%s\t%s\t%s\n", source.Name, len(data), sha, filepath.Join(sourceSnapshotDir, source.File), source.URL); err != nil {
			return err
		}
	}

	if err := os.WriteFile(manifestPath, []byte(manifest.String()), 0o644); err != nil {
		return err
	}

	fmt.Printf("bench corpus: %s (%d sources, %d bytes)\n", corpusPath, len(sources), totalBytes)
	return nil
}

func readOrDownload(ctx context.Context, client *http.Client, source sourceFile) ([]byte, error) {
	path := filepath.Join(sourceSnapshotDir, source.File)
	data, err := os.ReadFile(path)
	if err == nil {
		return normalize(data), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", source.Name, resp.Status)
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	data = normalize(data)
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("download %s: empty body", source.Name)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, err
	}
	return data, nil
}

func normalize(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(data, []byte("\r"), []byte("\n"))
}
