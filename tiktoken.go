package tiktoken

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dlclark/regexp2"
)

var bpeLoader BpeLoader = NewDefaultBpeLoader()

func SetBpeLoader(loader BpeLoader) {
	bpeLoader = loader
}

func GetEncoding(encodingName string) (*Tiktoken, error) {
	state, err := getEncodingState(encodingName)
	if err != nil {
		return nil, err
	}
	return NewTiktoken(state.core, state.encoding, state.specialTokensSet), nil
}

func EncodingForModel(modelName string) (*Tiktoken, error) {
	if encodingName, ok := MODEL_TO_ENCODING[modelName]; ok {
		return GetEncoding(encodingName)
	}
	for prefix, encodingName := range MODEL_PREFIX_TO_ENCODING {
		if strings.HasPrefix(modelName, prefix) {
			return GetEncoding(encodingName)
		}
	}
	return nil, fmt.Errorf("no encoding for model %s", modelName)
}

type Tiktoken struct {
	bpe              *CoreBPE
	pbeEncoding      *Encoding
	specialTokensSet map[string]any
}

func (t *Tiktoken) Encode(text string, allowedSpecial, disallowedSpecial []string) []int {
	var allowedSpecialSet map[string]any
	switch {
	case len(allowedSpecial) == 0:
		allowedSpecialSet = map[string]any{}
	case len(allowedSpecial) == 1 && allowedSpecial[0] == allowedSpecialAll:
		allowedSpecialSet = t.specialTokensSet
	default:
		allowedSpecialSet = map[string]any{}
		for _, v := range allowedSpecial {
			allowedSpecialSet[v] = nil
		}
	}

	disallowedSpecialSet := map[string]any{}
	for _, v := range disallowedSpecial {
		disallowedSpecialSet[v] = nil
	}
	if len(disallowedSpecial) == 1 && disallowedSpecial[0] == allowedSpecialAll {
		disallowedSpecialSet = difference(t.specialTokensSet, allowedSpecialSet)
	}

	if len(disallowedSpecialSet) > 0 {
		specialRegex := t.SpecialTokenRegex(disallowedSpecialSet)
		m := findRegex2StringMatch(text, specialRegex)
		if m != "" {
			panic("text contains disallowed special token " + m)
		}
	}

	if len(allowedSpecialSet) == 0 {
		return t.bpe.encodeOrdinaryNative(text)
	}

	tokens, _ := t.bpe.encodeNative(text, allowedSpecialSet)
	return tokens
}

func (t *Tiktoken) EncodeOrdinary(text string) []int {
	return t.bpe.encodeOrdinaryNative(text)
}

func (t *Tiktoken) Decode(tokens []int) string {
	return string(t.bpe.decodeNative(tokens))
}

func (t *Tiktoken) SpecialTokenRegex(disallowedSpecialSet map[string]any) *regexp2.Regexp {
	specialRegexStrs := make([]string, 0, len(disallowedSpecialSet))
	for k := range disallowedSpecialSet {
		specialRegexStrs = append(specialRegexStrs, regexp.QuoteMeta(k))
	}
	specialRegex := regexp2.MustCompile(strings.Join(specialRegexStrs, "|"), regexp2.None)
	return specialRegex
}

func findRegex2StringMatch(text string, reg *regexp2.Regexp) string {
	m, _ := reg.FindStringMatch(text)
	if m == nil {
		return ""
	}

	return m.String()
}

func difference(setA, setB map[string]any) map[string]any {
	result := make(map[string]any)
	for k := range setA {
		if _, ok := setB[k]; !ok {
			result[k] = true
		}
	}
	return result
}

// NewTiktoken can be used to create a *Tiktoken with custom parameters.
func NewTiktoken(bpe *CoreBPE, encoding *Encoding, specialTokensSet map[string]any) *Tiktoken {
	return &Tiktoken{
		bpe:              bpe,
		pbeEncoding:      encoding,
		specialTokensSet: specialTokensSet,
	}
}
