package tiktoken

import (
	"os"
	"strings"
	"testing"
)

const benchmarkInputPath = "testdata/token_inputs.txt"

var benchmarkEncodingSink []int

func readBenchmarkInput(b *testing.B) string {
	b.Helper()

	data, err := os.ReadFile(benchmarkInputPath)
	if err != nil {
		b.Fatal(err)
	}

	firstLine, _, _ := strings.Cut(string(data), "\n")
	return firstLine
}

func BenchmarkEncoding(b *testing.B) {
	tkm, err := EncodingForModel("gpt-4o")
	if err != nil {
		b.Fatal(err)
	}

	seed := readBenchmarkInput(b)
	benchmarks := []struct {
		name    string
		repeats int
	}{
		{name: "1x", repeats: 1},
		{name: "10x", repeats: 10},
		{name: "100x", repeats: 100},
	}

	for _, bm := range benchmarks {
		text := strings.Repeat(seed, bm.repeats)
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(text)))
			b.ResetTimer()

			for range b.N {
				benchmarkEncodingSink = tkm.Encode(text, nil, nil)
			}
		})
	}
}
