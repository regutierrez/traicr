package search

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	chunkBytes   = 16 * 1024
	chunkOverlap = 1024
)

func Chunks(text string) []string {
	if text == "" {
		return nil
	}
	chunks := make([]string, 0, len(text)/chunkBytes+1)
	for start := 0; start < len(text); {
		end := min(start+chunkBytes, len(text))
		for end < len(text) && !utf8.RuneStart(text[end]) {
			end--
		}
		chunks = append(chunks, text[start:end])
		if end == len(text) {
			break
		}
		start = end - min(chunkOverlap, end-start)
		for !utf8.RuneStart(text[start]) {
			start++
		}
	}
	return chunks
}

func Snippet(text, query string) string {
	if index := strings.Index(text, query); index >= 0 {
		return SnippetRange(text, index, index+len(query))
	}
	// Case folding can change byte lengths; find offsets in the original text.
	if pattern, err := regexp.Compile("(?i)" + regexp.QuoteMeta(query)); err == nil {
		if match := pattern.FindStringIndex(text); match != nil {
			return SnippetRange(text, match[0], match[1])
		}
	}
	return SnippetRange(text, 0, 0)
}

func SnippetRange(text string, matchStart, matchEnd int) string {
	const radius = 120
	matchStart = max(0, min(matchStart, len(text)))
	matchEnd = max(matchStart, min(matchEnd, len(text)))
	start := max(0, matchStart-radius)
	// A regex can match the entire event, but a result must stay a small preview.
	end := min(len(text), matchEnd+radius, start+480)
	for start > 0 && !utf8.RuneStart(text[start]) {
		start++
	}
	for end < len(text) && !utf8.RuneStart(text[end]) {
		end--
	}
	return text[start:end]
}
