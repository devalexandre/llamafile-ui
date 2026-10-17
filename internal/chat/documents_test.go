package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type docBackend struct {
	Backend
	dir string
}

func (b docBackend) AttachmentPath(name string) string { return filepath.Join(b.dir, name) }

func TestWithDocuments(t *testing.T) {
	dir := t.TempDir()
	name := "1234__relatório.pdf" + DocumentSuffix
	if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.Repeat("a", 500)), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Service{backend: docBackend{dir: dir}}
	if DocumentName(name) != "relatório.pdf" || !IsDocument(name) || IsDocument("x.png") {
		t.Fatal("document name helpers")
	}
	got := s.withDocuments("resuma", []string{"img.png", name}, 100)
	if !strings.HasPrefix(got, "resuma\n\n<arquivo nome=\"relatório.pdf\">") || !strings.Contains(got, "truncado") {
		t.Fatalf("unexpected: %q", got)
	}
	if s.withDocuments("oi", []string{"img.png"}, 100) != "oi" {
		t.Fatal("images must not change the text")
	}
}
