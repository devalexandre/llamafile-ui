package extract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/devalexandre/llamafile-ui/internal/i18n"
)

// TestLocalesComplete fails when a message has no translation in some
// language or a translation breaks its placeholders. Run `go run ./tools/i18n`
// to list what is missing.
func TestLocalesComplete(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	keys, err := Keys(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) < 50 {
		t.Fatalf("only %d messages found: extractor broken?", len(keys))
	}
	for _, lang := range i18n.Languages {
		if lang.Code == i18n.Source {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, "internal", "i18n", "locales", lang.Code+".json"))
		if err != nil {
			t.Errorf("%s: %v", lang.Code, err)
			continue
		}
		dict := map[string]string{}
		if err := json.Unmarshal(raw, &dict); err != nil {
			t.Errorf("%s: invalid JSON: %v", lang.Code, err)
			continue
		}
		missing := 0
		for _, k := range keys {
			v, ok := dict[k]
			if !ok || v == "" {
				missing++
				if missing <= 3 {
					t.Errorf("%s: missing %q", lang.Code, k)
				}
				continue
			}
			if !reflect.DeepEqual(Placeholders(k), Placeholders(v)) {
				t.Errorf("%s: placeholders differ in %q → %q", lang.Code, k, v)
			}
		}
		if missing > 3 {
			t.Errorf("%s: %d messages without translation in total", lang.Code, missing)
		}
	}
}

func TestPlaceholders(t *testing.T) {
	got := Placeholders("Baixe %s · {size} em %.1f s (100%%)")
	want := []string{"%%", "%.1f", "%s", "{size}"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}
