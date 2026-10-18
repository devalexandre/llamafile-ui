package tts

import (
	"regexp"
	"strings"
)

var (
	thinkRe     = regexp.MustCompile(`(?s)<think>.*?</think>`)
	fenceRe     = regexp.MustCompile("(?s)```.*?```")
	imageRe     = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	linkRe      = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	urlRe       = regexp.MustCompile(`https?://\S+`)
	inlineRe    = regexp.MustCompile("`([^`]*)`")
	headingRe   = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s*`)
	quoteRe     = regexp.MustCompile(`(?m)^\s*>\s?`)
	bulletRe    = regexp.MustCompile(`(?m)^\s*([-*+]|\d+[.)])\s+`)
	ruleRe      = regexp.MustCompile(`(?m)^\s*([-*_]\s*){3,}$`)
	tableSepRe  = regexp.MustCompile(`(?m)^\s*\|?[\s:|-]+\|[\s:|-]*$`)
	emphasisRe  = regexp.MustCompile(`(\*\*|__|\*|_|~~)([^*_~\n]+)(\*\*|__|\*|_|~~)`)
	citationRe  = regexp.MustCompile(`\[\d+\]`)
	spacesRe    = regexp.MustCompile(`[ \t]+`)
	pipeRe      = regexp.MustCompile(`[ \t]*\|[ \t]*`)
	spacePunct  = regexp.MustCompile(` ([.,;:!?])`)
	blankLineRe = regexp.MustCompile(`\n{2,}`)
)

// Clean turns a Markdown answer into text worth hearing: code blocks, links,
// tables and formatting marks are dropped or simplified. Each line it returns
// is synthesized as one utterance.
func Clean(md string) string {
	s := thinkRe.ReplaceAllString(md, "")
	s = fenceRe.ReplaceAllString(s, "\n(trecho de código)\n")
	s = imageRe.ReplaceAllString(s, "")
	s = linkRe.ReplaceAllString(s, "$1")
	s = urlRe.ReplaceAllString(s, "")
	s = inlineRe.ReplaceAllString(s, "$1")
	s = tableSepRe.ReplaceAllString(s, "")
	s = ruleRe.ReplaceAllString(s, "")
	s = headingRe.ReplaceAllString(s, "")
	s = quoteRe.ReplaceAllString(s, "")
	s = bulletRe.ReplaceAllString(s, "")
	for i := 0; i < 2; i++ { // nested emphasis
		s = emphasisRe.ReplaceAllString(s, "$2")
	}
	s = citationRe.ReplaceAllString(s, "")
	s = pipeRe.ReplaceAllString(s, ", ")

	var lines []string
	for _, l := range strings.Split(blankLineRe.ReplaceAllString(s, "\n"), "\n") {
		l = spacePunct.ReplaceAllString(spacesRe.ReplaceAllString(l, " "), "$1")
		l = strings.Trim(l, " ,")
		if l != "" {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, "\n")
}
