package rag

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

var textExtensions = map[string]bool{
	".txt": true, ".md": true, ".markdown": true, ".rst": true, ".csv": true, ".tsv": true, ".json": true,
	".yaml": true, ".yml": true, ".toml": true, ".ini": true, ".xml": true, ".log": true, ".tex": true,
	".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true, ".java": true, ".kt": true,
	".c": true, ".h": true, ".cpp": true, ".hpp": true, ".cs": true, ".rs": true, ".rb": true, ".php": true,
	".swift": true, ".sh": true, ".sql": true, ".css": true, ".scss": true, ".vue": true, ".svelte": true,
	".lua": true, ".dart": true, ".scala": true, ".r": true, ".m": true,
}

// SupportedExtensions is shown to the user in file dialogs.
func SupportedExtensions() []string {
	out := []string{".pdf", ".docx", ".html", ".htm"}
	for ext := range textExtensions {
		out = append(out, ext)
	}
	return out
}

func Supported(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf", ".docx", ".html", ".htm":
		return true
	}
	return textExtensions[ext]
}

const maxFileSize = 50 << 20

// ExtractText reads a document and returns its plain text.
func ExtractText(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if st.Size() > maxFileSize {
		return "", fmt.Errorf("arquivo maior que %d MB", maxFileSize>>20)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		return extractPDF(path)
	case ".docx":
		return extractDOCX(path)
	case ".html", ".htm":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return stripHTML(string(data)), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", errors.New("o arquivo não é texto UTF-8")
	}
	return string(data), nil
}

func extractPDF(path string) (text string, err error) {
	defer func() {
		if r := recover(); r != nil { // the pdf library panics on some malformed files
			err = fmt.Errorf("não foi possível ler o PDF: %v", r)
		}
	}()
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("não foi possível abrir o PDF: %w", err)
	}
	defer f.Close()
	var b strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		rows, err := p.GetTextByRow()
		if err != nil {
			continue
		}
		for _, row := range rows {
			for _, w := range row.Content {
				b.WriteString(w.S)
			}
			b.WriteByte('\n')
		}
		b.WriteString("\n")
	}
	if strings.TrimSpace(b.String()) == "" {
		return "", errors.New("o PDF não tem texto extraível (talvez seja escaneado)")
	}
	return b.String(), nil
}

func extractDOCX(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return "", err
		}
		return docxText(data), nil
	}
	return "", errors.New("documento Word inválido")
}

func docxText(data []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var b strings.Builder
	inText := false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tab":
				b.WriteByte('\t')
			case "br":
				b.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				b.WriteString("\n\n")
			}
		case xml.CharData:
			if inText {
				b.Write(t)
			}
		}
	}
	return b.String()
}

var (
	scriptRe = regexp.MustCompile(`(?is)<(script|style|noscript)[^>]*>.*?</(script|style|noscript)>`)
	blockRe  = regexp.MustCompile(`(?i)</?(p|div|br|li|h[1-6]|tr|section|article)[^>]*>`)
	tagRe    = regexp.MustCompile(`<[^>]+>`)
)

func stripHTML(s string) string {
	s = scriptRe.ReplaceAllString(s, "")
	s = blockRe.ReplaceAllString(s, "\n")
	s = tagRe.ReplaceAllString(s, "")
	return html.UnescapeString(s)
}

var spacesRe = regexp.MustCompile(`[ \t]+`)
var blankLinesRe = regexp.MustCompile(`\n{3,}`)

func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = spacesRe.ReplaceAllString(s, " ")
	s = blankLinesRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// Chunk splits text into overlapping pieces of about size characters,
// preferring paragraph and sentence boundaries.
func Chunk(text string, size, overlap int) []string {
	text = normalize(text)
	if text == "" {
		return nil
	}
	var pieces []string
	for _, para := range strings.Split(text, "\n\n") {
		para = strings.TrimSpace(para)
		for utf8.RuneCountInString(para) > size {
			r := []rune(para)
			cut := size
			// Prefer cutting after a sentence end or a space.
			for i := size; i > size/2; i-- {
				if r[i-1] == '.' || r[i-1] == '!' || r[i-1] == '?' || r[i-1] == '\n' {
					cut = i
					break
				}
			}
			if cut == size {
				for i := size; i > size/2; i-- {
					if r[i-1] == ' ' {
						cut = i
						break
					}
				}
			}
			pieces = append(pieces, strings.TrimSpace(string(r[:cut])))
			para = strings.TrimSpace(string(r[cut:]))
		}
		if para != "" {
			pieces = append(pieces, para)
		}
	}

	var chunks []string
	var cur strings.Builder
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			chunks = append(chunks, s)
		}
		cur.Reset()
	}
	for _, p := range pieces {
		if cur.Len() > 0 && utf8.RuneCountInString(cur.String())+utf8.RuneCountInString(p)+2 > size {
			prev := []rune(cur.String())
			flush()
			if overlap > 0 && len(prev) > overlap {
				tail := string(prev[len(prev)-overlap:])
				if i := strings.IndexAny(tail, " \n"); i >= 0 {
					tail = tail[i+1:]
				}
				cur.WriteString(tail)
				cur.WriteString(" … ")
			}
		}
		if cur.Len() > 0 {
			cur.WriteString("\n\n")
		}
		cur.WriteString(p)
	}
	flush()
	return chunks
}
