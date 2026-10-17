package rag

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestChunk(t *testing.T) {
	para := strings.Repeat("Frase de teste com algumas palavras. ", 20) // ~740 chars
	text := strings.Join([]string{para, para, para, "Fim."}, "\n\n")
	chunks := Chunk(text, 1200, 200)
	if len(chunks) < 2 {
		t.Fatalf("expected several chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if n := utf8.RuneCountInString(c); n > 1200+250 {
			t.Errorf("chunk %d too long: %d", i, n)
		}
	}
	if !strings.Contains(chunks[len(chunks)-1], "Fim.") {
		t.Error("last paragraph lost")
	}
	if Chunk("   \n\n ", 1200, 200) != nil {
		t.Error("blank text should produce no chunks")
	}
}

func TestDocxAndHTML(t *testing.T) {
	xml := `<w:document><w:body><w:p><w:r><w:t>Olá</w:t></w:r><w:r><w:t xml:space="preserve"> mundo</w:t></w:r></w:p><w:p><w:r><w:t>Segundo</w:t></w:r></w:p></w:body></w:document>`
	if got := strings.TrimSpace(docxText([]byte(xml))); got != "Olá mundo\n\nSegundo" {
		t.Errorf("docxText = %q", got)
	}
	html := `<html><head><style>p{}</style><script>x()</script></head><body><h1>Título</h1><p>Texto &amp; mais</p></body></html>`
	if got := normalize(stripHTML(html)); got != "Título\n\nTexto & mais" {
		t.Errorf("stripHTML = %q", got)
	}
}
