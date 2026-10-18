package imagegen

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareImg2Img(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ref.png")
	f, _ := os.Create(path)
	_ = png.Encode(f, image.NewRGBA(image.Rect(0, 0, 1200, 800)))
	f.Close()

	req := Request{InitImage: path, Width: 512, Height: 512, Steps: 1}
	if err := PrepareImg2Img(&req); err != nil {
		t.Fatal(err)
	}
	if req.Width != 512 || req.Height != 320 {
		t.Fatalf("size %dx%d, want 512x320 (keeps 3:2)", req.Width, req.Height)
	}
	if req.Strength != 0.6 || float64(req.Steps)*req.Strength < 2 {
		t.Fatalf("strength %.2f steps %d: turbo models need ≥2 denoising steps", req.Strength, req.Steps)
	}
	if err := PrepareImg2Img(&Request{InitImage: filepath.Join(t.TempDir(), "x.png")}); err == nil {
		t.Fatal("missing file must fail")
	}
}

func TestPrepareVideo(t *testing.T) {
	req := Request{Video: true, Frames: 30, Width: 832, Height: 480}
	if err := prepareVideo(&req); err != nil {
		t.Fatal(err)
	}
	if req.Frames != 29 || req.FPS != 16 || req.Width != 832 {
		t.Fatalf("frames %d fps %d size %dx%d", req.Frames, req.FPS, req.Width, req.Height)
	}
}
