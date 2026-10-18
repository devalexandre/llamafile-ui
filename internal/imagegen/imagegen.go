// Package imagegen generates images with diffusionfile (stable-diffusion.cpp)
// and keeps a gallery in SQLite.
package imagegen

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/devalexandre/llamafile-ui/internal/i18n"
	"github.com/devalexandre/llamafile-ui/internal/tools"
)

type Request struct {
	ModelID    string  `json:"modelId"`
	Prompt     string  `json:"prompt"`
	Negative   string  `json:"negative"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Steps      int     `json:"steps"`
	CFG        float64 `json:"cfg"`
	Seed       int64   `json:"seed"` // < 0 = random
	Sampler    string  `json:"sampler"`
	FastDecode bool    `json:"fastDecode"` // use TAESD when available
	// InitImage is a reference image to transform (img2img); empty = text-to-image.
	InitImage string  `json:"initImage"`
	Strength  float64 `json:"strength"` // how much the reference may change (0..1)
	// Video generates a clip (Wan models): Frames must be 4n+1. With a
	// reference image it becomes the first frame (image-to-video).
	Video  bool   `json:"video"`
	Frames int    `json:"frames"`
	FPS    int    `json:"fps"`
	Format string `json:"format"` // webm (default), webp (animated) or avi
}

type Image struct {
	ID         int64   `json:"id"`
	Prompt     string  `json:"prompt"`
	Negative   string  `json:"negative"`
	ModelID    string  `json:"modelId"`
	ModelName  string  `json:"modelName"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Steps      int     `json:"steps"`
	CFG        float64 `json:"cfg"`
	Seed       int64   `json:"seed"`
	Sampler    string  `json:"sampler"`
	File       string  `json:"file"`
	DurationMs int64   `json:"durationMs"`
	CreatedAt  int64   `json:"createdAt"`
	Reference  string  `json:"reference"` // copy of the reference image (img2img), if any
	Strength   float64 `json:"strength"`
	Kind       string  `json:"kind"` // image | video
	Frames     int     `json:"frames"`
	FPS        int     `json:"fps"`
}

type Progress struct {
	JobID   string  `json:"jobId"`
	Phase   string  `json:"phase"` // loading, encoding, sampling, decoding, saving
	Step    int     `json:"step"`
	Steps   int     `json:"steps"`
	Elapsed float64 `json:"elapsed"`
	ETA     float64 `json:"eta"` // seconds, estimated
}

// ModelSpec is what the generator needs to know about a model.
type ModelSpec struct {
	Name       string
	MainFlag   string // model | diffusion-model
	MainPath   string
	Components map[string]string // role → path
	Args       []string
}

type Backend interface {
	Binary() string
	Model(id string) (ModelSpec, error)
	Estimate(req Request) float64 // seconds
}

type Events struct {
	Progress func(Progress)
	Done     func(jobID string, img Image)
	Error    func(jobID string, err string)
}

type Generator struct {
	db      *sql.DB
	dir     string
	backend Backend
	events  Events

	mu     sync.Mutex
	jobID  string
	cancel context.CancelFunc
}

const schema = `
CREATE TABLE IF NOT EXISTS images (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    prompt      TEXT NOT NULL,
    negative    TEXT NOT NULL DEFAULT '',
    model_id    TEXT NOT NULL,
    model_name  TEXT NOT NULL DEFAULT '',
    width       INTEGER NOT NULL,
    height      INTEGER NOT NULL,
    steps       INTEGER NOT NULL,
    cfg         REAL NOT NULL,
    seed        INTEGER NOT NULL,
    sampler     TEXT NOT NULL DEFAULT '',
    file        TEXT NOT NULL,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL
);`

func New(db *sql.DB, dir string, backend Backend, events Events) (*Generator, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("images db: %w", err)
	}
	// Columns added after the first release (errors mean they already exist).
	_, _ = db.Exec(`ALTER TABLE images ADD COLUMN reference TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE images ADD COLUMN strength REAL NOT NULL DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE images ADD COLUMN kind TEXT NOT NULL DEFAULT 'image'`)
	_, _ = db.Exec(`ALTER TABLE images ADD COLUMN frames INTEGER NOT NULL DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE images ADD COLUMN fps INTEGER NOT NULL DEFAULT 0`)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Generator{db: db, dir: dir, backend: backend, events: events}, nil
}

// Busy returns the running job id, if any.
func (g *Generator) Busy() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.jobID
}

// Generate starts a job in the background and returns its id.
func (g *Generator) Generate(req Request) (string, error) {
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		return "", errors.New(i18n.T("descreva o que você quer gerar"))
	}
	bin := g.backend.Binary()
	if bin == "" {
		return "", errors.New(i18n.T("nenhum motor de imagens instalado"))
	}
	spec, err := g.backend.Model(req.ModelID)
	if err != nil {
		return "", err
	}
	if req.Width <= 0 || req.Height <= 0 {
		req.Width, req.Height = 512, 512
	}
	req.Width, req.Height = req.Width/64*64, req.Height/64*64
	if req.Steps <= 0 {
		req.Steps = 20
	}
	switch {
	case req.Video:
		if err := prepareVideo(&req); err != nil {
			return "", err
		}
	case req.InitImage != "":
		if err := PrepareImg2Img(&req); err != nil {
			return "", err
		}
	}
	if req.Seed < 0 {
		req.Seed = rand.Int64N(1 << 31)
	}

	g.mu.Lock()
	if g.jobID != "" {
		g.mu.Unlock()
		return "", errors.New(i18n.T("já existe uma imagem sendo gerada"))
	}
	jobID := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	g.jobID, g.cancel = jobID, cancel
	g.mu.Unlock()

	go func() {
		defer func() {
			g.mu.Lock()
			g.jobID, g.cancel = "", nil
			g.mu.Unlock()
			cancel()
		}()
		img, err := g.run(ctx, jobID, bin, spec, req)
		switch {
		case ctx.Err() != nil:
			if g.events.Error != nil {
				g.events.Error(jobID, "")
			}
		case err != nil:
			if g.events.Error != nil {
				g.events.Error(jobID, err.Error())
			}
		default:
			if g.events.Done != nil {
				g.events.Done(jobID, img)
			}
		}
	}()
	return jobID, nil
}

func (g *Generator) Cancel() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cancel != nil {
		g.cancel()
	}
}

var stepRe = regexp.MustCompile(`(\d+)/(\d+)\s*-\s*[\d.]+\s*(s/it|it/s)`)

// splitCRLF splits on \n and \r so progress bars are seen as they update.
func splitCRLF(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func (g *Generator) run(ctx context.Context, jobID, bin string, spec ModelSpec, req Request) (Image, error) {
	ext := ".png"
	var args []string
	if req.Video {
		ext = "." + req.Format
		// Tiled VAE decode avoids running out of VRAM on 4–8 GB cards.
		args = []string{"-M", "vid_gen", "--video-frames", strconv.Itoa(req.Frames), "--fps", strconv.Itoa(req.FPS), "--vae-tiling"}
	}
	name := uuid.NewString() + ext
	out := filepath.Join(g.dir, name)
	args = append(args, "--"+spec.MainFlag, spec.MainPath,
		"-p", req.Prompt, "-W", strconv.Itoa(req.Width), "-H", strconv.Itoa(req.Height),
		"--steps", strconv.Itoa(req.Steps), "--cfg-scale", strconv.FormatFloat(req.CFG, 'f', -1, 64),
		"-s", strconv.FormatInt(req.Seed, 10), "-o", out, "-t", strconv.Itoa(tools.Threads()))
	if req.Negative != "" {
		args = append(args, "-n", req.Negative)
	}
	reference := ""
	if req.InitImage != "" {
		args = append(args, "-i", req.InitImage)
		if !req.Video { // for video the reference is the first frame
			args = append(args, "--strength", strconv.FormatFloat(req.Strength, 'f', 2, 64))
		}
		// Keep a copy so the gallery can show what the image came from.
		reference = "ref-" + uuid.NewString() + strings.ToLower(filepath.Ext(req.InitImage))
		if err := copyFile(req.InitImage, filepath.Join(g.dir, reference)); err != nil {
			reference = ""
		}
	}
	if req.Sampler != "" {
		args = append(args, "--sampling-method", req.Sampler)
	}
	for role, path := range spec.Components {
		if role == "taesd" {
			if req.FastDecode {
				args = append(args, "--taesd", path)
			}
			continue
		}
		args = append(args, "--"+role, path)
	}
	args = append(args, spec.Args...)

	started := time.Now()
	estimate := g.backend.Estimate(req)
	emit := func(phase string, step, steps int) {
		if g.events.Progress == nil {
			return
		}
		elapsed := time.Since(started).Seconds()
		g.events.Progress(Progress{JobID: jobID, Phase: phase, Step: step, Steps: steps, Elapsed: elapsed,
			ETA: max(0, estimate-elapsed)})
	}
	emit("loading", 0, req.Steps)

	cmd := tools.Command(ctx, bin, args...)
	cmd.Dir = g.dir
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return Image{}, err
	}
	var tail []string
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	sc.Split(splitCRLF)
	phase := "loading"
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		switch {
		case strings.Contains(line, "get_learned_condition") || strings.Contains(line, "computing condition"):
			phase = "encoding"
			emit(phase, 0, req.Steps)
		case strings.Contains(line, "generating image:") || strings.Contains(line, "generate_video "):
			phase = "sampling"
			emit(phase, 0, req.Steps)
		case strings.Contains(line, "sampling completed") ||
			(strings.Contains(line, "decoding") && strings.Contains(line, "latent")):
			phase = "decoding"
			emit(phase, req.Steps, req.Steps)
		case strings.Contains(line, "save result"):
			emit("saving", req.Steps, req.Steps)
		}
		if phase == "sampling" {
			if m := stepRe.FindStringSubmatch(line); m != nil {
				step, _ := strconv.Atoi(m[1])
				total, _ := strconv.Atoi(m[2])
				emit(phase, step, total)
			}
		}
		tail = append(tail, line)
		if len(tail) > 15 {
			tail = tail[1:]
		}
	}
	if err := cmd.Wait(); err != nil {
		_ = os.Remove(out)
		if ctx.Err() != nil {
			return Image{}, ctx.Err()
		}
		return Image{}, errors.New(i18n.Tf("o motor de imagens falhou: %s", lastError(tail)))
	}
	if _, err := os.Stat(out); err != nil {
		return Image{}, errors.New(i18n.Tf("a imagem não foi gerada: %s", lastError(tail)))
	}

	img := Image{Prompt: req.Prompt, Negative: req.Negative, ModelID: req.ModelID, ModelName: spec.Name,
		Width: req.Width, Height: req.Height, Steps: req.Steps, CFG: req.CFG, Seed: req.Seed, Sampler: req.Sampler,
		File: name, DurationMs: time.Since(started).Milliseconds(), CreatedAt: time.Now().UnixMilli(), Reference: reference, Kind: "image"}
	if reference != "" && !req.Video {
		img.Strength = req.Strength
	}
	if req.Video {
		img.Kind, img.Frames, img.FPS = "video", req.Frames, req.FPS
	}
	res, err := g.db.Exec(`INSERT INTO images (prompt, negative, model_id, model_name, width, height, steps, cfg, seed, sampler, file, duration_ms, created_at, reference, strength, kind, frames, fps)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, img.Prompt, img.Negative, img.ModelID, img.ModelName, img.Width, img.Height,
		img.Steps, img.CFG, img.Seed, img.Sampler, img.File, img.DurationMs, img.CreatedAt, img.Reference, img.Strength, img.Kind, img.Frames, img.FPS)
	if err != nil {
		return img, err
	}
	img.ID, _ = res.LastInsertId()
	return img, nil
}

func lastError(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		l := lines[i]
		if strings.HasPrefix(l, "[E]") || strings.Contains(l, "ERROR") || strings.Contains(strings.ToLower(l), "error") {
			// stable-diffusion.cpp: "[E] message --- file.cpp:12"
			l = strings.TrimSpace(strings.TrimPrefix(l, "[E]"))
			if i := strings.Index(l, " --- "); i > 0 {
				l = l[:i]
			}
			return l
		}
	}
	if len(lines) > 0 {
		return strings.TrimSpace(lines[len(lines)-1])
	}
	return i18n.T("erro desconhecido")
}

func (g *Generator) List() ([]Image, error) {
	rows, err := g.db.Query(`SELECT id, prompt, negative, model_id, model_name, width, height, steps, cfg, seed, sampler, file, duration_ms, created_at, reference, strength, kind, frames, fps
FROM images ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Image{}
	for rows.Next() {
		var i Image
		if err := rows.Scan(&i.ID, &i.Prompt, &i.Negative, &i.ModelID, &i.ModelName, &i.Width, &i.Height, &i.Steps,
			&i.CFG, &i.Seed, &i.Sampler, &i.File, &i.DurationMs, &i.CreatedAt, &i.Reference, &i.Strength, &i.Kind, &i.Frames, &i.FPS); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (g *Generator) Get(id int64) (Image, error) {
	var i Image
	err := g.db.QueryRow(`SELECT id, prompt, negative, model_id, model_name, width, height, steps, cfg, seed, sampler, file, duration_ms, created_at, reference, strength, kind, frames, fps
FROM images WHERE id = ?`, id).Scan(&i.ID, &i.Prompt, &i.Negative, &i.ModelID, &i.ModelName, &i.Width, &i.Height, &i.Steps,
		&i.CFG, &i.Seed, &i.Sampler, &i.File, &i.DurationMs, &i.CreatedAt, &i.Reference, &i.Strength, &i.Kind, &i.Frames, &i.FPS)
	return i, err
}

func (g *Generator) Path(file string) string { return filepath.Join(g.dir, filepath.Base(file)) }

func (g *Generator) Delete(id int64) error {
	img, err := g.Get(id)
	if err != nil {
		return err
	}
	_ = os.Remove(g.Path(img.File))
	if img.Reference != "" {
		_ = os.Remove(g.Path(img.Reference))
	}
	_, err = g.db.Exec(`DELETE FROM images WHERE id = ?`, id)
	return err
}
