//go:build integration

package imagegen

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/devalexandre/llamafile-ui/internal/paths"
	"github.com/devalexandre/llamafile-ui/internal/tools"
)

type backend struct{}

func (backend) Binary() string { p, _ := tools.Find(tools.Diffusion, ""); return p }
func (backend) Model(string) (ModelSpec, error) {
	dir := paths.DefaultModelsDir()
	return ModelSpec{Name: "SD-Turbo", MainFlag: "model", MainPath: filepath.Join(dir, "sd_turbo-f16-q8_0.gguf"),
		Components: map[string]string{"taesd": filepath.Join(dir, "taesd.safetensors")}}, nil
}
func (backend) Estimate(Request) float64 { return 15 }

// go test -tags integration ./internal/imagegen -v
func TestIntegrationGenerate(t *testing.T) {
	if (backend{}).Binary() == "" {
		t.Skip("diffusionfile not installed")
	}
	db, _ := sql.Open("sqlite", filepath.Join(t.TempDir(), "img.db"))
	done := make(chan Image, 1)
	fail := make(chan string, 1)
	var phases []string
	g, err := New(db, t.TempDir(), backend{}, Events{
		Progress: func(p Progress) {
			if len(phases) == 0 || phases[len(phases)-1] != p.Phase {
				phases = append(phases, p.Phase)
			}
		},
		Done:  func(_ string, img Image) { done <- img },
		Error: func(_ string, e string) { fail <- e },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Generate(Request{ModelID: "sd-turbo", Prompt: "a lighthouse at sunset, oil painting", Width: 512, Height: 512,
		Steps: 1, CFG: 1, Seed: 3, Sampler: "euler_a", FastDecode: true}); err != nil {
		t.Fatal(err)
	}
	select {
	case img := <-done:
		t.Logf("ok %s in %dms phases=%v", img.File, img.DurationMs, phases)
	case e := <-fail:
		t.Fatal(e)
	case <-time.After(4 * time.Minute):
		t.Fatal("timeout")
	}
}
