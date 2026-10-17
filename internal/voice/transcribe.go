package voice

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/devalexandre/llamafile-ui/internal/tools"
)

// Supported by whisperfile without conversion.
var SupportedExtensions = []string{".wav", ".mp3", ".ogg", ".flac"}

func Supported(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range SupportedExtensions {
		if e == ext {
			return true
		}
	}
	return false
}

type Options struct {
	Binary   string // whisperfile path
	Model    string // ggml-*.bin path
	Language string // "auto", "pt", "en"…
	Threads  int
	// OnProgress receives 0..100.
	OnProgress func(percent int)
}

type Result struct {
	Text string `json:"text"`
	SRT  string `json:"srt"`
}

var progressRe = regexp.MustCompile(`progress\s*=\s*(\d+)%`)

// Transcribe runs whisperfile on an audio file and returns text and SRT subtitles.
func Transcribe(ctx context.Context, audio string, opt Options) (Result, error) {
	if !Supported(audio) {
		return Result{}, fmt.Errorf("formato não suportado (%s): use WAV, MP3, OGG ou FLAC", filepath.Ext(audio))
	}
	if opt.Binary == "" {
		return Result{}, errors.New("whisperfile não instalado")
	}
	if opt.Model == "" {
		return Result{}, errors.New("nenhum modelo de voz instalado")
	}
	lang := opt.Language
	if lang == "" {
		lang = "auto"
	}
	threads := opt.Threads
	if threads <= 0 {
		threads = tools.Threads()
	}

	tmp, err := os.MkdirTemp("", "llamafile-ui-whisper-*")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(tmp)
	out := filepath.Join(tmp, "out")

	cmd := tools.Command(ctx, opt.Binary, "-m", opt.Model, "-f", audio, "-l", lang,
		"-t", strconv.Itoa(threads), "-otxt", "-osrt", "-of", out, "-pp", "-np", "-sns")
	stderr, _ := cmd.StderrPipe()
	cmd.Stdout = io.Discard
	if err := cmd.Start(); err != nil {
		return Result{}, err
	}
	var tail []string
	sc := bufio.NewScanner(stderr)
	for sc.Scan() {
		line := sc.Text()
		if m := progressRe.FindStringSubmatch(line); m != nil && opt.OnProgress != nil {
			p, _ := strconv.Atoi(m[1])
			opt.OnProgress(p)
		}
		tail = append(tail, line)
		if len(tail) > 20 {
			tail = tail[1:]
		}
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return Result{}, fmt.Errorf("whisperfile falhou: %s", strings.TrimSpace(strings.Join(tail, " ")))
	}
	text, err := os.ReadFile(out + ".txt")
	if err != nil {
		return Result{}, errors.New("whisperfile não gerou a transcrição")
	}
	srt, _ := os.ReadFile(out + ".srt")
	return Result{Text: cleanText(string(text)), SRT: string(srt)}, nil
}

// nonSpeechRe matches whisper annotations such as [BLANK_AUDIO], [Music] or (risos).
var nonSpeechRe = regexp.MustCompile(`^[\[(（][^\])）]*[\])）][.!]?$`)

// cleanText joins whisper segments into readable paragraphs.
func cleanText(s string) string {
	var parts []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || nonSpeechRe.MatchString(line) {
			continue
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, " ")
}

var srtEndRe = regexp.MustCompile(`--> (\d+):(\d+):(\d+)[,.](\d+)`)

// AudioSeconds returns the length of an audio file: from the WAV header when
// possible, otherwise from the last subtitle timestamp (0 if unknown).
func AudioSeconds(path, srt string) float64 {
	if strings.EqualFold(filepath.Ext(path), ".wav") {
		if f, err := os.Open(path); err == nil {
			defer f.Close()
			hdr := make([]byte, 44)
			if _, err := io.ReadFull(f, hdr); err == nil && string(hdr[0:4]) == "RIFF" && string(hdr[36:40]) == "data" {
				byteRate := binary.LittleEndian.Uint32(hdr[28:32])
				size := binary.LittleEndian.Uint32(hdr[40:44])
				if byteRate > 0 {
					return float64(size) / float64(byteRate)
				}
			}
		}
	}
	m := srtEndRe.FindAllStringSubmatch(srt, -1)
	if len(m) == 0 {
		return 0
	}
	last := m[len(m)-1]
	h, _ := strconv.Atoi(last[1])
	mi, _ := strconv.Atoi(last[2])
	s, _ := strconv.Atoi(last[3])
	ms, _ := strconv.Atoi(last[4])
	return float64(h*3600+mi*60+s) + float64(ms)/1000
}
