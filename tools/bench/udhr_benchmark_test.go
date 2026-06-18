package bench

import (
	"errors"
	"os"
	"strings"
	"testing"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

const udhrInputPath = "/tmp/udhr.txt"

var ordinaryBenchmarkSink []int

// go test -benchmem -run=^$ -bench ^BenchmarkEncodingInFullLanguage$ -benchtime=100000x ./tools/bench
func BenchmarkEncodingInFullLanguage(b *testing.B) {
	data, err := os.ReadFile(udhrInputPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			b.Skipf("%s missing; run tools/legacy-python/get_udhr.py first", udhrInputPath)
		}
		b.Fatal(err)
	}

	lines := strings.Split(string(data), "\n")
	tkm, err := tiktoken.EncodingForModel("gpt-4o")
	if err != nil {
		b.Fatal(err)
	}

	lineCount := len(lines)
	b.ReportAllocs()
	b.ResetTimer()
	for n := range b.N {
		ordinaryBenchmarkSink = tkm.EncodeOrdinary(lines[n%lineCount])
	}
}
