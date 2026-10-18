// Package catalog holds the curated model list, what is installed locally and
// ranks models for the current machine.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/devalexandre/llamafile-ui/internal/hardware"
)

//go:embed catalog.json
var catalogJSON []byte

const (
	RuntimeLlamafile = "llamafile"
	RuntimeWhisper   = "whisperfile"
	RuntimeDiffusion = "diffusionfile"
	RuntimePiper     = "piper"
)

type File struct {
	Role  string `json:"role,omitempty"` // components only: vae, clip_l, t5xxl, llm, taesd
	Quant string `json:"quant,omitempty"`
	File  string `json:"file"`
	URL   string `json:"url"`
	Size  int64  `json:"size"`
}

// ImageSpec holds the defaults to run an image model with diffusionfile.
type ImageSpec struct {
	MainFlag         string   `json:"main_flag"` // "model" (all-in-one) or "diffusion-model"
	Steps            int      `json:"steps"`
	CFG              float64  `json:"cfg"`
	Width            int      `json:"width"`
	Height           int      `json:"height"`
	Sampler          string   `json:"sampler"`
	DiffusionParamsB float64  `json:"diffusion_params_b"`
	VAECost          float64  `json:"vae_cost"` // relative VAE decode cost (SD=1)
	Args             []string `json:"args,omitempty"`
}

// VideoSpec holds the defaults of a video model (Wan) run with vid_gen.
type VideoSpec struct {
	Frames     int  `json:"frames"`      // 4n+1
	FPS        int  `json:"fps"`         // playback rate
	ImageInput bool `json:"image_input"` // accepts a first-frame reference image (I2V)
}

// TTSSpec describes a Piper voice.
type TTSSpec struct {
	Language string `json:"language"` // BCP 47, e.g. "pt-BR"
}

// EmbedSpec holds the prefixes some embedding models expect.
type EmbedSpec struct {
	QueryPrefix string `json:"query_prefix"`
	DocPrefix   string `json:"doc_prefix"`
}

type Model struct {
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
	Files         []File             `json:"files"`
	MMProj        *File              `json:"mmproj,omitempty"`
	Components    []File             `json:"components,omitempty"`
	Image         *ImageSpec         `json:"image,omitempty"`
	Embed         *EmbedSpec         `json:"embed,omitempty"`
	TTS           *TTSSpec           `json:"tts,omitempty"`
	Video         *VideoSpec         `json:"video,omitempty"`
}

// HasCategory reports whether the model belongs to category.
func (m *Model) HasCategory(category string) bool { return slices.Contains(m.Categories, category) }

// IsChat reports whether the model can be used in the chat.
func (m *Model) IsChat() bool { return m.Runtime == RuntimeLlamafile && !m.HasCategory("embedding") }

// extras are the files needed besides the main weights.
func (m *Model) extras() []File {
	var out []File
	if m.MMProj != nil {
		out = append(out, *m.MMProj)
	}
	return append(out, m.Components...)
}

func (m *Model) extrasSize() int64 {
	var n int64
	for _, f := range m.extras() {
		n += f.Size
	}
	return n
}

// View is what the UI renders: catalog data + fit for this machine + local state.
type View struct {
	Model
	Fit            Fit    `json:"fit"`
	Installed      bool   `json:"installed"`
	InstalledQuant string `json:"installedQuant"`
	InstalledBytes int64  `json:"installedBytes"`
	Local          bool   `json:"local"` // a .gguf found on disk that is not in the catalog
	Vision         bool   `json:"vision"`
}

type Category struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

var Categories = []Category{
	{"chat", "Chat", "Assistentes para conversar, escrever e resumir"},
	{"code", "Código", "Gerar, explicar e refatorar código"},
	{"reasoning", "Raciocínio", "Lógica, matemática e problemas passo a passo"},
	{"vision", "Visão", "Entendem imagens, capturas de tela e documentos"},
	{"image", "Imagens", "Geram imagens a partir de texto"},
	{"video", "Vídeos", "Geram vídeos curtos a partir de texto ou de uma imagem"},
	{"voice", "Voz", "Transcrição de áudio com Whisper"},
	{"embedding", "Embeddings", "Busca semântica nos seus documentos"},
	{"tts", "Leitura em voz alta", "Vozes para ouvir as respostas do chat (Piper)"},
}

type Catalog struct {
	models    []Model
	modelsDir func() string
	cal       Calibrator
}

// SetCalibrator applies speeds measured on this machine to every evaluation.
func (c *Catalog) SetCalibrator(cal Calibrator) { c.cal = cal }

func New(modelsDir func() string) (*Catalog, error) {
	var doc struct {
		Models []Model `json:"models"`
	}
	if err := json.Unmarshal(catalogJSON, &doc); err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	return &Catalog{models: doc.Models, modelsDir: modelsDir}, nil
}

func (c *Catalog) Find(id string) *Model {
	for i := range c.models {
		if c.models[i].ID == id {
			return &c.models[i]
		}
	}
	if name, ok := strings.CutPrefix(id, "local:"); ok {
		return c.localModel(name)
	}
	return nil
}

func (c *Catalog) path(name string) string { return filepath.Join(c.modelsDir(), name) }

func exists(path string) (int64, bool) {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return 0, false
	}
	return st.Size(), true
}

// InstalledFile returns the first quant of m present on disk, provided every
// extra file (mmproj, components) is present too.
func (c *Catalog) InstalledFile(m *Model) (File, bool) {
	for _, f := range m.extras() {
		if _, ok := exists(c.path(f.File)); !ok {
			return File{}, false
		}
	}
	for _, f := range m.Files {
		if _, ok := exists(c.path(f.File)); ok {
			return f, true
		}
	}
	return File{}, false
}

func (c *Catalog) ModelPath(id string) string {
	m := c.Find(id)
	if m == nil {
		return ""
	}
	if f, ok := c.InstalledFile(m); ok {
		return c.path(f.File)
	}
	return ""
}

func (c *Catalog) MMProjPath(id string) string {
	m := c.Find(id)
	if m == nil || m.MMProj == nil {
		return ""
	}
	if _, ok := exists(c.path(m.MMProj.File)); ok {
		return c.path(m.MMProj.File)
	}
	return ""
}

// ComponentPaths maps component roles to local paths (installed ones only).
func (c *Catalog) ComponentPaths(id string) map[string]string {
	out := map[string]string{}
	m := c.Find(id)
	if m == nil {
		return out
	}
	for _, f := range m.Components {
		if _, ok := exists(c.path(f.File)); ok {
			out[f.Role] = c.path(f.File)
		}
	}
	return out
}

func (c *Catalog) DisplayName(id string) string {
	if m := c.Find(id); m != nil {
		return m.Name
	}
	return id
}

func (c *Catalog) view(m *Model, category string, hw hardware.Info, ctx int) View {
	v := View{Model: *m, Vision: m.MMProj != nil, Local: strings.HasPrefix(m.ID, "local:")}
	if f, ok := c.InstalledFile(m); ok {
		v.Installed = true
		v.InstalledQuant = f.Quant
		v.InstalledBytes = f.Size + m.extrasSize()
		v.Fit = EvaluateWith(m, f, category, hw, ctx, c.cal)
	} else {
		v.Fit = EvaluateBestWith(m, category, hw, ctx, c.cal)
	}
	return v
}

func primaryCategory(m *Model) string {
	if len(m.Categories) > 0 {
		return m.Categories[0]
	}
	return "chat"
}

// Recommended ranks catalog models of a category for this machine.
// category "all" ranks every model by its primary category.
func (c *Catalog) Recommended(category string, hw hardware.Info, ctx int) []View {
	out := make([]View, 0, len(c.models))
	for i := range c.models {
		m := &c.models[i]
		cat := category
		if category == "all" {
			cat = primaryCategory(m)
		} else if !m.HasCategory(category) {
			continue
		}
		out = append(out, c.view(m, cat, hw, ctx))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Fit.Score > out[j].Fit.Score })
	return out
}

// TopPicks returns the best model that fits for each category.
func (c *Catalog) TopPicks(hw hardware.Info, ctx int) map[string]View {
	picks := map[string]View{}
	for _, cat := range Categories {
		for _, v := range c.Recommended(cat.ID, hw, ctx) {
			if v.Fit.Level != "tight" {
				picks[cat.ID] = v
				break
			}
		}
	}
	return picks
}

// Installed lists catalog models on disk plus any other .gguf in the models dir.
func (c *Catalog) Installed(hw hardware.Info, ctx int) []View {
	out := []View{}
	known := map[string]bool{}
	for i := range c.models {
		m := &c.models[i]
		for _, f := range m.Files {
			known[f.File] = true
		}
		for _, f := range m.extras() {
			known[f.File] = true
		}
		if _, ok := c.InstalledFile(m); ok {
			out = append(out, c.view(m, primaryCategory(m), hw, ctx))
		}
	}
	entries, _ := os.ReadDir(c.modelsDir())
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || known[name] || !strings.HasSuffix(strings.ToLower(name), ".gguf") ||
			strings.HasPrefix(strings.ToLower(name), "mmproj") {
			continue
		}
		if m := c.localModel(name); m != nil {
			out = append(out, c.view(m, "chat", hw, ctx))
		}
	}
	return out
}

// InstalledFor lists installed models of a category (e.g. voice, image, embedding).
func (c *Catalog) InstalledFor(category string, hw hardware.Info, ctx int) []View {
	out := []View{}
	for _, v := range c.Installed(hw, ctx) {
		if v.HasCategory(category) {
			out = append(out, v)
		}
	}
	return out
}

func (c *Catalog) localModel(name string) *Model {
	size, ok := exists(c.path(name))
	if !ok {
		return nil
	}
	// Without metadata, assume ~4.5 bits per weight to estimate parameters.
	params := float64(size) * 8 / 4.8 / 1e9
	return &Model{
		ID:            "local:" + name,
		Name:          strings.TrimSuffix(name, filepath.Ext(name)),
		Publisher:     "Arquivo local",
		ParamsB:       params,
		ActiveParamsB: params,
		Context:       8192,
		KVMBPer1K:     128,
		Categories:    []string{"chat"},
		Quality:       map[string]float64{"chat": 55},
		Runtime:       RuntimeLlamafile,
		Description:   "Modelo GGUF adicionado manualmente à pasta de modelos.",
		Files:         []File{{Quant: "GGUF", File: name, Size: size}},
	}
}

// DownloadPlan returns the files to fetch for a model (weights + extras).
func (c *Catalog) DownloadPlan(id, quant string, hw hardware.Info, ctx int) (*Model, []File, error) {
	m := c.Find(id)
	if m == nil || strings.HasPrefix(id, "local:") {
		return nil, nil, fmt.Errorf("modelo %q não encontrado no catálogo", id)
	}
	if quant == "" {
		quant = EvaluateBestWith(m, primaryCategory(m), hw, ctx, c.cal).Quant
	}
	var files []File
	for _, f := range m.Files {
		if f.Quant == quant {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("quantização %q não disponível para %s", quant, m.Name)
	}
	return m, append(files, m.extras()...), nil
}

func (c *Catalog) Dest(f File) string { return c.path(f.File) }

// Remove deletes the files of a model (all quants, extras, partial downloads).
// Extras shared with another installed model (e.g. a VAE) are kept.
func (c *Catalog) Remove(id string) error {
	m := c.Find(id)
	if m == nil {
		return fmt.Errorf("modelo %q não encontrado", id)
	}
	shared := map[string]bool{}
	for i := range c.models {
		other := &c.models[i]
		if other.ID == m.ID {
			continue
		}
		if _, ok := c.InstalledFile(other); !ok {
			continue
		}
		for _, f := range other.extras() {
			shared[f.File] = true
		}
	}
	for _, f := range append(append([]File{}, m.Files...), m.extras()...) {
		if shared[f.File] {
			continue
		}
		_ = os.Remove(c.path(f.File))
		_ = os.Remove(c.path(f.File) + ".part")
	}
	return nil
}
