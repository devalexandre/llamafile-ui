// Command catalog builds internal/catalog/catalog.json from the curated
// tools/catalog/models.json, filling download URLs and real file sizes from
// the Hugging Face API.
//
//	go run ./tools/catalog
//
// Quality scores in models.json are editorial (0-100 per category).
// kv_mb_per_1k = 2 * layers * kv_heads * head_dim * 2 bytes per 1024 tokens (MiB).
// Components are extra files a model needs (VAE, text encoders, TAESD…); they
// may live in other repos and can be renamed locally with "local" so that
// models sharing the same file download it only once.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"
)

type sourceQuant struct {
	Quant string `json:"quant"`
	File  string `json:"file"`
}

type sourceComponent struct {
	Role  string `json:"role"`
	Repo  string `json:"repo"`
	File  string `json:"file"`
	Local string `json:"local,omitempty"`
}

type sourceModel struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Publisher     string             `json:"publisher"`
	ParamsB       float64            `json:"params_b"`
	ActiveParamsB float64            `json:"active_params_b,omitempty"`
	Context       int                `json:"context"`
	KVMBPer1K     float64            `json:"kv_mb_per_1k"`
	Categories    []string           `json:"categories"`
	Quality       map[string]float64 `json:"quality"`
	Runtime       string             `json:"runtime,omitempty"`
	Repo          string             `json:"repo"`
	License       string             `json:"license"`
	Tags          []string           `json:"tags"`
	Description   string             `json:"description"`
	Quants        []sourceQuant      `json:"quants"`
	MMProj        string             `json:"mmproj,omitempty"`
	Components    []sourceComponent  `json:"components,omitempty"`
	Image         json.RawMessage    `json:"image,omitempty"`
	Embed         json.RawMessage    `json:"embed,omitempty"`
	TTS           json.RawMessage    `json:"tts,omitempty"`
}

type file struct {
	Role  string `json:"role,omitempty"`
	Quant string `json:"quant,omitempty"`
	File  string `json:"file"`
	URL   string `json:"url"`
	Size  int64  `json:"size"`
}

type model struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Publisher     string             `json:"publisher"`
	ParamsB       float64            `json:"params_b"`
	ActiveParamsB float64            `json:"active_params_b"`
	Context       int                `json:"context"`
	KVMBPer1K     float64            `json:"kv_mb_per_1k"`
	Categories    []string           `json:"categories"`
	Quality       map[string]float64 `json:"quality"`
	Runtime       string             `json:"runtime"`
	Repo          string             `json:"repo"`
	License       string             `json:"license"`
	Tags          []string           `json:"tags"`
	Description   string             `json:"description"`
	Files         []file             `json:"files"`
	MMProj        *file              `json:"mmproj,omitempty"`
	Components    []file             `json:"components,omitempty"`
	Image         json.RawMessage    `json:"image,omitempty"`
	Embed         json.RawMessage    `json:"embed,omitempty"`
	TTS           json.RawMessage    `json:"tts,omitempty"`
}

type treeEntry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	LFS  *struct {
		Size int64 `json:"size"`
	} `json:"lfs"`
}

var client = &http.Client{Timeout: 60 * time.Second}

type sizer struct{ cache map[string]map[string]int64 }

// size returns the size of a file; name may live in a subfolder of the repo
// (e.g. Piper voices: "pt/pt_BR/faber/medium/pt_BR-faber-medium.onnx").
func (s *sizer) size(repo, name string) int64 {
	dir := path.Dir(name)
	if dir == "." {
		dir = ""
	}
	key := repo + "/" + dir
	sizes, ok := s.cache[key]
	if !ok {
		log.Printf("fetching %s", key)
		var err error
		if sizes, err = fetchSizes(repo, dir); err != nil {
			log.Fatal(err)
		}
		s.cache[key] = sizes
	}
	size, ok := sizes[name]
	if !ok {
		log.Fatalf("file %s not found in %s", name, repo)
	}
	return size
}

func fetchSizes(repo, dir string) (map[string]int64, error) {
	u := "https://huggingface.co/api/models/" + repo + "/tree/main"
	if dir != "" {
		u += "/" + (&url.URL{Path: dir}).EscapedPath()
	}
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", repo, resp.StatusCode)
	}
	var tree []treeEntry
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, err
	}
	sizes := make(map[string]int64, len(tree))
	for _, e := range tree {
		if e.LFS != nil {
			sizes[e.Path] = e.LFS.Size
		} else {
			sizes[e.Path] = e.Size
		}
	}
	return sizes, nil
}

func resolveURL(repo, name string) string {
	return "https://huggingface.co/" + repo + "/resolve/main/" + (&url.URL{Path: name}).EscapedPath()
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "tools", "catalog", "models.json"))
	if err != nil {
		log.Fatal(err)
	}
	var sources []sourceModel
	if err := json.Unmarshal(raw, &sources); err != nil {
		log.Fatal(err)
	}

	sz := &sizer{cache: map[string]map[string]int64{}}
	out := make([]model, 0, len(sources))
	for _, s := range sources {
		m := model{
			ID: s.ID, Name: s.Name, Publisher: s.Publisher, ParamsB: s.ParamsB, ActiveParamsB: s.ActiveParamsB,
			Context: s.Context, KVMBPer1K: s.KVMBPer1K, Categories: s.Categories, Quality: s.Quality,
			Runtime: s.Runtime, Repo: s.Repo, License: s.License, Tags: s.Tags, Description: s.Description,
			Image: s.Image, Embed: s.Embed, TTS: s.TTS,
		}
		if m.Runtime == "" {
			m.Runtime = "llamafile"
		}
		if m.ActiveParamsB == 0 {
			m.ActiveParamsB = m.ParamsB
		}
		for _, q := range s.Quants {
			// Files are saved flat in the models folder.
			m.Files = append(m.Files, file{Quant: q.Quant, File: path.Base(q.File), URL: resolveURL(s.Repo, q.File), Size: sz.size(s.Repo, q.File)})
		}
		if s.MMProj != "" {
			m.MMProj = &file{File: s.MMProj, URL: resolveURL(s.Repo, s.MMProj), Size: sz.size(s.Repo, s.MMProj)}
		}
		for _, c := range s.Components {
			local := c.Local
			if local == "" {
				local = path.Base(c.File)
			}
			m.Components = append(m.Components, file{Role: c.Role, File: local, URL: resolveURL(c.Repo, c.File), Size: sz.size(c.Repo, c.File)})
		}
		out = append(out, m)
	}

	data, err := json.MarshalIndent(map[string]any{"version": 2, "models": out}, "", " ")
	if err != nil {
		log.Fatal(err)
	}
	dest := filepath.Join(root, "internal", "catalog", "catalog.json")
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %d models to %s", len(out), dest)
}
