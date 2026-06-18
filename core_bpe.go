package tiktoken

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

const asciiFastPathMinBytes = 1024

type corePatternKind uint8

const (
	corePatternGeneric corePatternKind = iota
	corePatternO200k
)

type CoreBPE struct {
	encoder              map[string]int
	decoder              map[int]string
	specialTokensEncoder map[string]int
	specialTokensDecoder map[int]string
	tlRegex              *regexp2.Regexp
	tlSpecialRegex       *regexp2.Regexp
	patternKind          corePatternKind
}

func NewCoreBPE(encoder, specialTokensEncoder map[string]int, pattern string) (*CoreBPE, error) {
	regex, err := regexp2.Compile(pattern, regexp2.None)
	if err != nil {
		return nil, fmt.Errorf("error compiling regex: %w", err)
	}

	specialRegexStrs := make([]string, 0, len(specialTokensEncoder))
	for k := range specialTokensEncoder {
		specialRegexStrs = append(specialRegexStrs, regexp.QuoteMeta(k))
	}
	specialRegex, err := regexp2.Compile(strings.Join(specialRegexStrs, "|"), regexp2.None)
	if err != nil {
		return nil, fmt.Errorf("error compiling special regex: %w", err)
	}

	decoder := make(map[int]string, len(encoder))
	for k, v := range encoder {
		decoder[v] = k
	}

	if len(encoder) != len(decoder) {
		return nil, errors.New("encoder and decoder map sizes are different")
	}

	specialTokensDecoder := make(map[int]string, len(specialTokensEncoder))
	for k, v := range specialTokensEncoder {
		specialTokensDecoder[v] = k
	}

	return &CoreBPE{
		encoder:              encoder,
		specialTokensEncoder: specialTokensEncoder,
		decoder:              decoder,
		specialTokensDecoder: specialTokensDecoder,
		tlRegex:              regex,
		tlSpecialRegex:       specialRegex,
		patternKind:          detectCorePattern(pattern),
	}, nil
}

//nolint:gocognit // Regex/special-token loop mirrors upstream logic; refactoring risks tokenizer parity.
func (bp *CoreBPE) encodeNative(text string, allowedSpecial map[string]any) ([]int, int) {
	if len(text) >= asciiFastPathMinBytes && isASCII(text) {
		return bp.encodeNativeASCII(text, allowedSpecial)
	}
	specialRegex := bp.tlSpecialRegex
	regex := bp.tlRegex
	ret := []int{}
	lastPieceTokenLen := 0
	textRunes := []rune(text)

	start := 0
	for {
		var nextSpecial []int
		startFind := start
		for {
			// Find the next allowed special token, if any
			temp := cutRunes(textRunes, startFind, len(textRunes))
			nextSpecial = findRegex2StringIndex(temp, specialRegex)
			if nextSpecial != nil {
				token := cutRunes(textRunes, startFind+nextSpecial[0], startFind+nextSpecial[1])
				if _, ok := allowedSpecial[token]; ok {
					break
				}
				startFind += nextSpecial[1]
			} else {
				break
			}
		}

		end := len(textRunes)
		if nextSpecial != nil {
			end = start + nextSpecial[0]
		}

		// Okay, here we go, compare this logic to _encode_ordinary_native
		forEachRegex2StringMatchIndex(cutRunes(textRunes, start, end), regex, func(matchStart, matchEnd int) {
			piece := cutRunes(textRunes, start+matchStart, start+matchEnd)
			if token, ok := bp.encoder[piece]; ok {
				lastPieceTokenLen = 1
				ret = append(ret, token)
				return
			}
			tokens := bytePairEncode(piece, bp.encoder)
			lastPieceTokenLen = len(tokens)
			ret = append(ret, tokens...)
		})

		if nextSpecial != nil {
			temp := cutRunes(textRunes, start+nextSpecial[0], start+nextSpecial[1])
			token := bp.specialTokensEncoder[temp]
			ret = append(ret, token)
			start += nextSpecial[1]
			lastPieceTokenLen = 0
		} else {
			break
		}
	}

	return ret, lastPieceTokenLen
}

func (bp *CoreBPE) encodeNativeASCII(text string, allowedSpecial map[string]any) ([]int, int) {
	specialRegex := bp.tlSpecialRegex
	regex := bp.tlRegex
	ret := []int{}
	lastPieceTokenLen := 0

	start := 0
	for {
		var nextSpecial []int
		startFind := start
		for {
			nextSpecial = findRegex2StringIndex(text[startFind:], specialRegex)
			if nextSpecial != nil {
				token := text[startFind+nextSpecial[0] : startFind+nextSpecial[1]]
				if _, ok := allowedSpecial[token]; ok {
					break
				}
				startFind += nextSpecial[1]
			} else {
				break
			}
		}

		end := len(text)
		if nextSpecial != nil {
			end = start + nextSpecial[0]
		}

		forEachRegex2StringMatchIndex(text[start:end], regex, func(matchStart, matchEnd int) {
			piece := text[start+matchStart : start+matchEnd]
			if token, ok := bp.encoder[piece]; ok {
				lastPieceTokenLen = 1
				ret = append(ret, token)
				return
			}
			tokens := bytePairEncode(piece, bp.encoder)
			lastPieceTokenLen = len(tokens)
			ret = append(ret, tokens...)
		})

		if nextSpecial != nil {
			temp := text[start+nextSpecial[0] : start+nextSpecial[1]]
			token := bp.specialTokensEncoder[temp]
			ret = append(ret, token)
			start += nextSpecial[1]
			lastPieceTokenLen = 0
		} else {
			break
		}
	}

	return ret, lastPieceTokenLen
}

func (bp *CoreBPE) encodeOrdinaryNative(text string) []int {
	if isASCII(text) && (len(text) >= asciiFastPathMinBytes || bp.patternKind == corePatternO200k) {
		return bp.encodeOrdinaryNativeASCII(text)
	}
	if len(text) >= asciiFastPathMinBytes && bp.patternKind == corePatternO200k {
		return bp.encodeOrdinaryNativeO200kHybrid(text)
	}
	ret := []int{}
	textRunes := []rune(text)
	forEachRegex2StringMatchIndex(text, bp.tlRegex, func(start, end int) {
		piece := cutRunes(textRunes, start, end)
		ret = bp.appendEncodedPiece(ret, piece)
	})
	return ret
}

func (bp *CoreBPE) encodeOrdinaryNativeASCII(text string) []int {
	ret := []int{}
	if bp.patternKind == corePatternO200k {
		forEachO200kASCIIMatchIndex(text, func(start, end int) {
			piece := text[start:end]
			ret = bp.appendEncodedPiece(ret, piece)
		})
		return ret
	}
	forEachRegex2StringMatchIndex(text, bp.tlRegex, func(start, end int) {
		piece := text[start:end]
		ret = bp.appendEncodedPiece(ret, piece)
	})
	return ret
}

func (bp *CoreBPE) encodeOrdinaryNativeO200kHybrid(text string) []int {
	ret := []int{}
	start := 0
	for start < len(text) {
		nonASCII := nextNonASCIIByte(text, start)
		if nonASCII < 0 {
			forEachO200kASCIIMatchIndex(text[start:], func(matchStart, matchEnd int) {
				ret = bp.appendEncodedPiece(ret, text[start+matchStart:start+matchEnd])
			})
			break
		}

		islandStart := o200kNonASCIIIslandStart(text, start, nonASCII)
		if islandStart > start {
			forEachO200kASCIIMatchIndex(text[start:islandStart], func(matchStart, matchEnd int) {
				ret = bp.appendEncodedPiece(ret, text[start+matchStart:start+matchEnd])
			})
		}

		islandEnd := o200kNonASCIIIslandEnd(text, nonASCII)
		ret = bp.appendRegexEncodedMatches(ret, text[islandStart:islandEnd])
		start = islandEnd
	}
	return ret
}

func (bp *CoreBPE) appendRegexEncodedMatches(ret []int, text string) []int {
	textRunes := []rune(text)
	forEachRegex2StringMatchIndex(text, bp.tlRegex, func(start, end int) {
		piece := cutRunes(textRunes, start, end)
		ret = bp.appendEncodedPiece(ret, piece)
	})
	return ret
}

func (bp *CoreBPE) appendEncodedPiece(ret []int, piece string) []int {
	if token, ok := bp.encoder[piece]; ok {
		return append(ret, token)
	}
	tokens := bytePairEncode(piece, bp.encoder)
	return append(ret, tokens...)
}

func detectCorePattern(pattern string) corePatternKind {
	if pattern == o200kPatStr {
		return corePatternO200k
	}
	return corePatternGeneric
}

func isASCII(text string) bool {
	for i := range len(text) {
		if text[i] >= 0x80 {
			return false
		}
	}
	return true
}

func nextNonASCIIByte(text string, start int) int {
	for i := start; i < len(text); i++ {
		if text[i] >= utf8.RuneSelf {
			return i
		}
	}
	return -1
}

func o200kNonASCIIIslandStart(text string, floor, nonASCII int) int {
	start := nonASCII
	for start > floor && !isASCIIWhitespace(text[start-1]) {
		start--
	}
	for start > floor && isASCIIWhitespace(text[start-1]) {
		start--
	}
	return start
}

func o200kNonASCIIIslandEnd(text string, nonASCII int) int {
	end := nonASCII
	for end < len(text) {
		if text[end] < utf8.RuneSelf {
			if isASCIIWhitespace(text[end]) {
				break
			}
			end++
			continue
		}

		_, size := utf8.DecodeRuneInString(text[end:])
		if size == 0 {
			return end
		}
		end += size
	}
	for end < len(text) && (text[end] == '\r' || text[end] == '\n' || text[end] == '/') {
		end++
	}
	return end
}

func (bpe *CoreBPE) decodeNative(tokens []int) []byte {
	ret := make([]byte, 0, len(tokens)*2)
	for _, token := range tokens {
		tokenBytes, ok := bpe.decoder[token]
		if !ok {
			tokenBytes = bpe.specialTokensDecoder[token]
		}
		if tokenBytes != "" {
			ret = append(ret, tokenBytes...)
		}
	}
	return ret
}

func findRegex2StringIndex(text string, reg *regexp2.Regexp) []int {
	m, _ := reg.FindStringMatch(text)
	if m == nil {
		return nil
	}
	result := make([]int, 2)
	result[0] = m.Index
	result[1] = m.Index + m.Length
	return result
}

func forEachRegex2StringMatchIndex(text string, reg *regexp2.Regexp, fn func(start, end int)) {
	m, _ := reg.FindStringMatch(text)
	for m != nil {
		fn(m.Index, m.Index+m.Length)
		m, _ = reg.FindNextMatch(m)
	}
}

func forEachO200kASCIIMatchIndex(text string, fn func(start, end int)) {
	for start := 0; start < len(text); {
		end := nextO200kASCIIMatchEnd(text, start)
		fn(start, end)
		start = end
	}
}

func nextO200kASCIIMatchEnd(text string, start int) int {
	if end, ok := nextO200kASCIIWordEnd(text, start); ok {
		return end
	}

	if isASCIIDigit(text[start]) {
		end := start + 1
		for end < len(text) && end-start < 3 && isASCIIDigit(text[end]) {
			end++
		}
		return end
	}

	if end, ok := nextO200kASCIIPunctuationEnd(text, start); ok {
		return end
	}

	if isASCIIWhitespace(text[start]) {
		return nextO200kASCIIWhitespaceEnd(text, start)
	}

	return start + 1
}

func nextO200kASCIIWordEnd(text string, start int) (int, bool) {
	wordStart := start
	if isO200kASCIIWordPrefix(text[start]) {
		if start+1 >= len(text) || !isASCIILetter(text[start+1]) {
			return 0, false
		}
		wordStart++
	}

	if !isASCIILetter(text[wordStart]) {
		return 0, false
	}

	if end, ok := nextO200kLowerWordEnd(text, wordStart); ok {
		return consumeO200kASCIIContraction(text, end), true
	}
	if end, ok := nextO200kUpperWordEnd(text, wordStart); ok {
		return consumeO200kASCIIContraction(text, end), true
	}

	return 0, false
}

func nextO200kLowerWordEnd(text string, start int) (int, bool) {
	end := start
	for end < len(text) && isASCIIUpper(text[end]) {
		end++
	}

	lowerStart := end
	for end < len(text) && isASCIILower(text[end]) {
		end++
	}
	if end == lowerStart {
		return 0, false
	}
	return end, true
}

func nextO200kUpperWordEnd(text string, start int) (int, bool) {
	end := start
	for end < len(text) && isASCIIUpper(text[end]) {
		end++
	}
	if end == start {
		return 0, false
	}
	for end < len(text) && isASCIILower(text[end]) {
		end++
	}
	return end, true
}

func nextO200kASCIIPunctuationEnd(text string, start int) (int, bool) {
	pos := start
	if text[pos] == ' ' {
		if pos+1 >= len(text) || !isO200kASCIIPunctuation(text[pos+1]) {
			return 0, false
		}
		pos++
	}

	if !isO200kASCIIPunctuation(text[pos]) {
		return 0, false
	}
	for pos < len(text) && isO200kASCIIPunctuation(text[pos]) {
		pos++
	}
	for pos < len(text) && (text[pos] == '\r' || text[pos] == '\n' || text[pos] == '/') {
		pos++
	}
	return pos, true
}

func nextO200kASCIIWhitespaceEnd(text string, start int) int {
	end := start
	lastNewlineEnd := -1
	for end < len(text) && isASCIIWhitespace(text[end]) {
		if text[end] == '\r' || text[end] == '\n' {
			lastNewlineEnd = end + 1
		}
		end++
	}
	if lastNewlineEnd >= 0 {
		return lastNewlineEnd
	}
	if end == len(text) || end-start == 1 {
		return end
	}
	return end - 1
}

func consumeO200kASCIIContraction(text string, start int) int {
	if start >= len(text) || text[start] != '\'' {
		return start
	}
	if hasASCIIInsensitivePrefix(text[start:], "'re") ||
		hasASCIIInsensitivePrefix(text[start:], "'ve") ||
		hasASCIIInsensitivePrefix(text[start:], "'ll") {
		return start + 3
	}
	if start+2 <= len(text) {
		switch toASCIILower(text[start+1]) {
		case 's', 't', 'm', 'd':
			return start + 2
		}
	}
	return start
}

func hasASCIIInsensitivePrefix(text, prefix string) bool {
	if len(text) < len(prefix) {
		return false
	}
	for i := range len(prefix) {
		if toASCIILower(text[i]) != prefix[i] {
			return false
		}
	}
	return true
}

func isO200kASCIIWordPrefix(b byte) bool {
	return b != '\r' && b != '\n' && !isASCIILetter(b) && !isASCIIDigit(b)
}

func isO200kASCIIPunctuation(b byte) bool {
	return !isASCIIWhitespace(b) && !isASCIILetter(b) && !isASCIIDigit(b)
}

func isASCIILetter(b byte) bool {
	return isASCIIUpper(b) || isASCIILower(b)
}

func isASCIIUpper(b byte) bool {
	return b >= 'A' && b <= 'Z'
}

func isASCIILower(b byte) bool {
	return b >= 'a' && b <= 'z'
}

func isASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isASCIIWhitespace(b byte) bool {
	switch b {
	case '\t', '\n', '\v', '\f', '\r', ' ':
		return true
	default:
		return false
	}
}

func toASCIILower(b byte) byte {
	if isASCIIUpper(b) {
		return b + ('a' - 'A')
	}
	return b
}

func cutRunes(runes []rune, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[start:end])
}
