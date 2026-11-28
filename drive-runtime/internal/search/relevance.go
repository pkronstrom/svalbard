package search

import (
	"math/bits"
	"strings"
	"unicode"
)

// MatchesQueryWord matches a preview token to a query token, including common
// singular/plural spellings. It keeps unrelated words with a shared prefix apart.
func MatchesQueryWord(word, term string) bool {
	word, term = strings.ToLower(word), strings.ToLower(term)
	return word == term ||
		(len(term) >= 4 && (word == term+"s" || word == term+"es")) ||
		(len(word) >= 4 && (term == word+"s" || term == word+"es"))
}

// GrowingQuery is a supplemental phrasing for short English farming searches.
// How-to guides often use "growing X", while "X farming" finds farms and trade
// classifications. Searches for either word by itself keep their usual meaning.
func GrowingQuery(words []string) []string {
	if len(words) == 2 {
		for i, word := range words {
			if strings.EqualFold(word, "farming") {
				return []string{"growing", words[1-i]}
			}
		}
	}
	return nil
}

// PassageEvidence counts distinct terms and looks for all of them within eight
// words, in either order. Snippet punctuation and FTS highlight delimiters
// do not affect the match.
func PassageEvidence(text string, words []string) (count int, nearby bool) {
	if text == "" || len(words) == 0 {
		return 0, false
	}
	tokens := strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	const windowSize = 8
	var all uint16
	var recent [windowSize]uint16
	target := uint16(1<<len(words)) - 1
	for i, token := range tokens {
		var mask uint16
		for i, word := range words {
			if MatchesQueryWord(token, word) {
				mask |= 1 << i
			}
		}
		recent[i%windowSize] = mask
		all |= mask
		if !nearby && mask != 0 && all == target {
			var window uint16
			for _, term := range recent {
				window |= term
			}
			nearby = window == target
		}
	}
	return bits.OnesCount16(all), nearby
}
