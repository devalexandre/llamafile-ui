package chat

import (
	"fmt"
	"github.com/devalexandre/llamafile-ui/internal/i18n"
	"os"
	"strings"
	"unicode/utf8"
)

// DocumentSuffix marks attachments holding the text extracted from a file
// (PDF, DOCX, code…) attached to a message; images keep their extension.
const DocumentSuffix = ".doc.txt"

// IsDocument reports whether an attachment is extracted document text.
func IsDocument(name string) bool { return strings.HasSuffix(name, DocumentSuffix) }

// DocumentName is the original file name of a document attachment
// ("<uuid>__report.pdf.doc.txt" → "report.pdf").
func DocumentName(name string) string {
	name = strings.TrimSuffix(name, DocumentSuffix)
	if _, after, ok := strings.Cut(name, "__"); ok {
		return after
	}
	return name
}

func hasImages(attachments []string) bool {
	for _, a := range attachments {
		if !IsDocument(a) {
			return true
		}
	}
	return false
}

// withDocuments appends the text of attached documents to a user message,
// truncated to limit characters in total.
func (s *Service) withDocuments(text string, attachments []string, limit int) string {
	var b strings.Builder
	for _, a := range attachments {
		if !IsDocument(a) {
			continue
		}
		data, err := os.ReadFile(s.backend.AttachmentPath(a))
		if err != nil {
			continue
		}
		doc := string(data)
		if utf8.RuneCountInString(doc) > limit {
			doc = string([]rune(doc)[:max(limit, 0)]) + "\n" + i18n.T("[… arquivo truncado para caber no contexto]")
		}
		limit -= utf8.RuneCountInString(doc)
		fmt.Fprintf(&b, "\n\n<arquivo nome=%q>\n%s\n</arquivo>", DocumentName(a), strings.TrimSpace(doc))
	}
	if b.Len() == 0 {
		return text
	}
	return strings.TrimSpace(text) + b.String()
}
