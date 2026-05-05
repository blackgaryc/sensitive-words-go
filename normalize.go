package sensitivewords

import (
	"strings"
	"unicode"
)

type normalizeMode int

const (
	normalizeStrict normalizeMode = iota
	normalizeCompact
)

type normalizedText struct {
	text       []rune
	original   []rune
	originRune []int
}

func normalizeInput(input string, mode normalizeMode) normalizedText {
	original := []rune(input)
	normalized := normalizedText{
		text:       make([]rune, 0, len(original)),
		original:   original,
		originRune: make([]int, 0, len(original)),
	}

	for idx, r := range original {
		r = foldRune(r)
		if shouldDropRune(r, mode) {
			continue
		}
		normalized.text = append(normalized.text, r)
		normalized.originRune = append(normalized.originRune, idx)
	}

	return normalized
}

func normalizeKeyword(input string, mode normalizeMode) string {
	normalized := normalizeInput(input, mode)
	return string(normalized.text)
}

func foldRune(r rune) rune {
	switch {
	case r == '\u3000':
		return ' '
	case r >= '\uFF01' && r <= '\uFF5E':
		r -= 0xFEE0
	}
	return unicode.ToLower(r)
}

func shouldDropRune(r rune, mode normalizeMode) bool {
	if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
		return true
	}

	if mode != normalizeCompact {
		return false
	}

	if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
		return true
	}

	return false
}

func sliceOriginalRunes(text normalizedText, startNorm, endNorm int) (startOriginal int, endOriginal int, matched string) {
	if startNorm < 0 || endNorm <= startNorm || endNorm > len(text.originRune) {
		return 0, 0, ""
	}

	startOriginal = text.originRune[startNorm]
	endOriginal = text.originRune[endNorm-1] + 1
	return startOriginal, endOriginal, string(text.original[startOriginal:endOriginal])
}

func runeLen(input string) int {
	return len([]rune(input))
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
