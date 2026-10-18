// Command i18n lists the translatable messages and what each locale lacks.
//
//	go run ./tools/i18n                 # summary
//	go run ./tools/i18n -missing es     # JSON object of untranslated messages for a language
//	go run ./tools/i18n -prune          # drop translations no longer used
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/devalexandre/llamafile-ui/internal/i18n"
	"github.com/devalexandre/llamafile-ui/internal/i18n/extract"
)

func main() {
	missingFor := flag.String("missing", "", "print the untranslated messages of a language as JSON")
	prune := flag.Bool("prune", false, "remove translations of messages that no longer exist")
	flag.Parse()

	keys, err := extract.Keys(".")
	if err != nil {
		log.Fatal(err)
	}
	for _, lang := range i18n.Languages {
		if lang.Code == i18n.Source {
			continue
		}
		path := filepath.Join("internal", "i18n", "locales", lang.Code+".json")
		dict := map[string]string{}
		if raw, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(raw, &dict)
		}
		missing := map[string]string{}
		for _, k := range keys {
			if dict[k] == "" {
				missing[k] = ""
			}
		}
		if *missingFor == lang.Code {
			out, _ := json.MarshalIndent(missing, "", "  ")
			fmt.Println(string(out))
			return
		}
		if *prune {
			used := map[string]bool{}
			for _, k := range keys {
				used[k] = true
			}
			for k := range dict {
				if !used[k] {
					delete(dict, k)
				}
			}
			if err := write(path, dict); err != nil {
				log.Fatal(err)
			}
		}
		if *missingFor == "" {
			fmt.Printf("%-6s %-22s %4d/%d traduzidas\n", lang.Code, lang.Name, len(keys)-len(missing), len(keys))
		}
	}
}

// write saves a locale with sorted keys and without HTML escaping.
func write(path string, dict map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(dict) // encoding/json sorts map keys
}
