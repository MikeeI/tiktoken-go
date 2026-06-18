package tiktoken

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/dlclark/regexp2"
	"github.com/stretchr/testify/assert"
)

func TestRegex2Func(t *testing.T) {
	ass := assert.New(t)
	pattern := `[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}`
	re := regexp.MustCompile(pattern)
	re2 := regexp2.MustCompile(pattern, regexp2.None)

	words := []string{
		"this is my email hi@google.com,and this is john's email world@outlook.com",
		"hi@google.com is email for google",
		"outlook email world@outlook.com is work for microsoft",
	}

	for _, word := range words {
		ass.Equal(re.FindStringIndex(word), findRegex2StringIndex(word, re2))
		var got [][]int
		forEachRegex2StringMatchIndex(word, re2, func(start, end int) {
			got = append(got, []int{start, end})
		})
		ass.Equal(re.FindAllStringSubmatchIndex(word, -1), got)
		ass.Equal(re.FindString(word), findRegex2StringMatch(word, re2))
	}
}

func TestO200kASCIIPatternMatchesRegexp2(t *testing.T) {
	re2 := regexp2.MustCompile(o200kPatStr, regexp2.None)
	cases := []string{
		"",
		"hello",
		" hello",
		"Hello",
		"HELLO",
		"aBC",
		"ABCdefG",
		"can't",
		"HELLO'S",
		"hello,world",
		" hello/world\n",
		"foo\nbar",
		"   a",
		"   ",
		"\t\tA",
		"...\n/",
		" .Hello",
		"1 2345",
		"abc123def",
		" \n \r  x",
		"snake_case HTTP2Parser::next_token",
		"if err != nil { return fmt.Errorf(\"wrap: %w\", err) }",
	}

	for _, tc := range cases {
		var got [][]int
		forEachO200kASCIIMatchIndex(tc, func(start, end int) {
			got = append(got, []int{start, end})
		})

		var want [][]int
		forEachRegex2StringMatchIndex(tc, re2, func(start, end int) {
			want = append(want, []int{start, end})
		})
		assert.Equal(t, want, got, tc)
	}
}

func TestO200kHybridEncodingMatchesRegexp2Path(t *testing.T) {
	tkm, err := EncodingForModel("gpt-4o")
	if err != nil {
		t.Fatal(err)
	}
	regexBPE := *tkm.bpe
	regexBPE.patternKind = corePatternGeneric

	cases := []string{
		strings.Repeat("package main\nfunc value() int { return 123 }\n", 1400) + " // “quoted” café\n" + strings.Repeat("fmt.Println(value())\n", 400),
		strings.Repeat("alpha beta gamma   ", 4000) + "überHTTP parser\n" + strings.Repeat("delta epsilon\n", 1200),
		strings.Repeat("if err != nil { return err }\n", 2200) + "\t// 你好, мир\n" + strings.Repeat("payload := []byte(\"ascii\")\n", 600),
	}

	for _, tc := range cases {
		assert.Equal(t, regexBPE.encodeOrdinaryNative(tc), tkm.bpe.encodeOrdinaryNative(tc))
	}
}

func TestO200kHybridEncodingMatchesRegexp2PathOnCorpus(t *testing.T) {
	data, err := os.ReadFile(githubCorpusPath)
	if err != nil {
		t.Fatal(err)
	}

	tkm, err := EncodingForModel("gpt-4o")
	if err != nil {
		t.Fatal(err)
	}
	regexBPE := *tkm.bpe
	regexBPE.patternKind = corePatternGeneric

	text := corpusPrefix(string(data), benchmark64KiBBytes)
	assert.Equal(t, regexBPE.encodeOrdinaryNative(text), tkm.bpe.encodeOrdinaryNative(text))
}
