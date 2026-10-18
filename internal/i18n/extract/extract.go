// Package extract finds every translatable message in the project: calls to
// i18n.T/Tf/N in Go, t("…") in the frontend and the user-facing texts of the
// model catalog.
package extract

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// jsCallRe matches t("…"), t('…'), t(`…`) (template literals without ${})
// and the msg("…") marker.
var jsCallRe = regexp.MustCompile("\\b(?:t|msg)\\(\\s*(\"(?:[^\"\\\\]|\\\\.)*\"|'(?:[^'\\\\]|\\\\.)*'|`(?:[^`\\\\$]|\\\\.)*`)")

// Keys returns the sorted, unique messages of the project at root.
func Keys(root string) ([]string, error) {
	set := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			switch name {
			case ".git", "node_modules", "bindings", "dist", "bin", "build", "tools", "docs":
				return filepath.SkipDir
			}
			return nil
		}
		switch {
		case strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go"):
			return goKeys(path, set)
		case (strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".tsx")) && strings.Contains(path, "frontend"+string(os.PathSeparator)+"src"):
			return jsKeys(path, set)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := catalogKeys(filepath.Join(root, "internal", "catalog", "catalog.json"), set); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(set))
	for k := range set {
		if strings.TrimSpace(k) != "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out, nil
}

func goKeys(path string, set map[string]bool) error {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return err
	}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "i18n" {
			return true
		}
		arg := 0
		switch sel.Sel.Name {
		case "T", "Tf", "N":
		case "TIn":
			arg = 1
		default:
			return true
		}
		if len(call.Args) > arg {
			if lit, ok := call.Args[arg].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, err := strconv.Unquote(lit.Value); err == nil {
					set[s] = true
				}
			}
		}
		return true
	})
	return nil
}

func jsKeys(path string, set map[string]bool) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, m := range jsCallRe.FindAllStringSubmatch(string(raw), -1) {
		set[unquoteJS(m[1])] = true
	}
	return nil
}

// unquoteJS decodes a JS string literal (the escapes used in this project).
func unquoteJS(lit string) string {
	body := lit[1 : len(lit)-1]
	r := strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\"`, `"`, `\'`, `'`, "\\`", "`", `\\`, `\`)
	return r.Replace(body)
}

func catalogKeys(path string, set map[string]bool) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc struct {
		Models []struct {
			Description string   `json:"description"`
			Tags        []string `json:"tags"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	for _, m := range doc.Models {
		set[m.Description] = true
		for _, t := range m.Tags {
			set[t] = true
		}
	}
	return nil
}

var placeholderRe = regexp.MustCompile(`%[-+#0]*\d*(?:\.\d+)?[sdvfqwxXtg%]|\{[a-zA-Z_][a-zA-Z0-9_]*\}`)

// Placeholders lists the fmt verbs and {name} placeholders of a message, sorted.
func Placeholders(s string) []string {
	p := placeholderRe.FindAllString(s, -1)
	sort.Strings(p)
	return p
}
