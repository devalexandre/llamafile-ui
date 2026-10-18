package chat

import (
	"strings"
	"unicode"
)

// RepetitionNote is appended when a looping answer is cut.
const RepetitionNote = "_(Resposta interrompida: o modelo começou a se repetir. Modelos muito pequenos fazem isso em respostas longas — experimente um modelo maior em Descobrir modelos.)_"

// repetitionCut detects a model stuck in a loop: some paragraph (30+ chars)
// already appeared twice and shows up a third time. It returns where to cut
// the content (the start of the third occurrence).
func repetitionCut(content string) (int, bool) {
	seen := map[string]int{}
	pos := 0
	for _, line := range strings.SplitAfter(content, "\n") {
		start := pos
		pos += len(line)
		key := normalizeParagraph(line)
		if len([]rune(key)) < 30 {
			continue
		}
		seen[key]++
		if seen[key] == 3 {
			return start, true
		}
	}
	return 0, false
}

// normalizeParagraph ignores case, punctuation, spacing and list markers.
func normalizeParagraph(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
