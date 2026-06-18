package tiktoken

import (
	"encoding/base64"
	"os"
	"strconv"
	"strings"
	"testing"
)

const (
	benchmarkInputPath  = "testdata/token_inputs.txt"
	githubCorpusPath    = "testdata/bench/github_corpus.txt"
	benchmark64KiBBytes = 64 << 10
	benchmark1MiBBytes  = 1 << 20
	benchmark4MiBBytes  = 4 << 20
)

var (
	benchmarkEncodingSink []int
	benchmarkTiktokenSink *Tiktoken
)

func readBenchmarkFixture(b *testing.B) string {
	b.Helper()

	data, err := os.ReadFile(benchmarkInputPath)
	if err != nil {
		b.Fatal(err)
	}

	firstLine, _, _ := strings.Cut(string(data), "\n")
	return firstLine
}

func readBenchmarkCorpus(b *testing.B) string {
	b.Helper()

	data, err := os.ReadFile(githubCorpusPath)
	if err != nil {
		b.Fatal(err)
	}

	return string(data)
}

func corpusPrefix(text string, maxBytes int) string {
	if len(text) <= maxBytes {
		return text
	}

	return text[:maxBytes]
}

func BenchmarkEncodingForModelLookup(b *testing.B) {
	b.ReportAllocs()

	for range b.N {
		tkm, err := EncodingForModel("gpt-4o")
		if err != nil {
			b.Fatal(err)
		}
		benchmarkTiktokenSink = tkm
	}
}

func BenchmarkLoadTiktokenBpeParsing(b *testing.B) {
	var builder strings.Builder
	for i := range 50_000 {
		token := "benchmark_token_" + strconv.Itoa(i)
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(token)))
		builder.WriteByte(' ')
		builder.WriteString(strconv.Itoa(i))
		builder.WriteByte('\n')
	}

	path := b.TempDir() + "/benchmark.tiktoken"
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := loadTiktokenBpe(path); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncoding(b *testing.B) {
	tkm, err := EncodingForModel("gpt-4o")
	if err != nil {
		b.Fatal(err)
	}

	githubCorpus := readBenchmarkCorpus(b)
	benchmarks := []struct {
		name string
		text string
	}{
		{name: "fixture", text: readBenchmarkFixture(b)},
		{name: "github/64KiB", text: corpusPrefix(githubCorpus, benchmark64KiBBytes)},
		{name: "github/1MiB", text: corpusPrefix(githubCorpus, benchmark1MiBBytes)},
		{name: "github/4MiB", text: corpusPrefix(githubCorpus, benchmark4MiBBytes)},
		{name: "adversarial/long_ascii_single_piece", text: strings.Repeat("abcdefghijklmnopqrstuvwxyz", 4096)},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(bm.text)))
			b.ResetTimer()

			for range b.N {
				benchmarkEncodingSink = tkm.Encode(bm.text, nil, nil)
			}
		})
	}
}
