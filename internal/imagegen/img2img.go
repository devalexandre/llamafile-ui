package imagegen

import (
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"

	"github.com/devalexandre/llamafile-ui/internal/i18n"
	_ "golang.org/x/image/webp"
)

// PrepareImg2Img validates a reference image request: the output keeps the
// reference aspect ratio (longest side = the model's default size, multiple
// of 64) and few-step models get enough steps to actually denoise.
func PrepareImg2Img(req *Request) error {
	f, err := os.Open(req.InitImage)
	if err != nil {
		return errors.New(i18n.T("não foi possível abrir a imagem de referência"))
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width == 0 || cfg.Height == 0 {
		return errors.New(i18n.T("formato de imagem de referência não suportado (use PNG, JPG, WebP ou GIF)"))
	}
	req.Width, req.Height = FitSize(cfg.Width, cfg.Height, max(req.Width, req.Height))
	if req.Strength <= 0 {
		req.Strength = 0.6
	}
	req.Strength = math.Min(math.Max(req.Strength, 0.05), 1)
	// stable-diffusion.cpp runs steps × strength denoising steps.
	if minSteps := int(math.Ceil(2 / req.Strength)); float64(req.Steps)*req.Strength < 2 {
		req.Steps = minSteps
	}
	return nil
}

// FitSize scales w×h so the longest side is longest, rounding to multiples of 64.
func FitSize(w, h, longest int) (int, int) {
	if longest <= 0 {
		longest = 512
	}
	scale := float64(longest) / float64(max(w, h))
	round := func(v float64) int { return max(64, int(math.Round(v/64))*64) }
	return round(float64(w) * scale), round(float64(h) * scale)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// prepareVideo normalizes a video request: Wan needs 4n+1 frames, and with a
// first-frame reference the output keeps its aspect ratio.
func prepareVideo(req *Request) error {
	if req.Frames <= 1 {
		req.Frames = 33
	}
	req.Frames = (req.Frames-1)/4*4 + 1
	if req.FPS <= 0 {
		req.FPS = 16
	}
	switch req.Format {
	case "webp", "avi":
	default:
		req.Format = "webm"
	}
	if req.InitImage != "" {
		f, err := os.Open(req.InitImage)
		if err != nil {
			return errors.New(i18n.T("não foi possível abrir a imagem de referência"))
		}
		cfg, _, err := image.DecodeConfig(f)
		f.Close()
		if err != nil || cfg.Width == 0 || cfg.Height == 0 {
			return errors.New(i18n.T("formato de imagem de referência não suportado (use PNG, JPG, WebP ou GIF)"))
		}
		req.Width, req.Height = FitSize(cfg.Width, cfg.Height, max(req.Width, req.Height))
	}
	return nil
}
