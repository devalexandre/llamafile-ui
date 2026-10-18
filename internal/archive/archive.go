// Package archive extracts downloaded .tar.gz and .zip packages safely
// (no path traversal, relative symlinks only).
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"github.com/devalexandre/llamafile-ui/internal/i18n"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Extract unpacks a .zip or .tar.gz archive into dest.
func Extract(archive, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	if strings.HasSuffix(strings.ToLower(archive), ".zip") {
		return extractZip(archive, dest)
	}
	return extractTarGz(archive, dest)
}

// target resolves an archive entry inside dest, rejecting path traversal.
func target(dest, name string) (string, error) {
	p := filepath.Join(dest, filepath.FromSlash(name))
	if !strings.HasPrefix(p, filepath.Clean(dest)+string(os.PathSeparator)) {
		return "", errors.New(i18n.Tf("caminho inválido no pacote: %s", name))
	}
	return p, nil
}

func extractTarGz(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		p, err := target(dest, h.Name)
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(p, 0o755)
		case tar.TypeSymlink:
			// Shared libraries are symlinked (libfoo.so → libfoo.so.1); keep them relative.
			if filepath.IsAbs(h.Linkname) || strings.Contains(h.Linkname, "..") {
				return errors.New(i18n.Tf("link inválido no pacote: %s", h.Name))
			}
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			err = os.Symlink(h.Linkname, p)
		case tar.TypeReg:
			err = writeFile(p, tr, os.FileMode(h.Mode)&0o777)
		}
		if err != nil {
			return err
		}
	}
}

func extractZip(archive, dest string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		p, err := target(dest, zf.Name)
		if err != nil {
			return err
		}
		if zf.Mode()&os.ModeSymlink != 0 {
			if err := zipSymlink(zf, p); err != nil {
				return err
			}
			continue
		}
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		err = writeFile(p, rc, zf.Mode().Perm()|0o600)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func writeFile(p string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// zipSymlink recreates a symlink stored in a zip (its content is the target).
func zipSymlink(zf *zip.File, p string) error {
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	link, err := io.ReadAll(io.LimitReader(rc, 4096))
	rc.Close()
	if err != nil {
		return err
	}
	target := string(link)
	if filepath.IsAbs(target) || strings.Contains(target, "..") {
		return errors.New(i18n.Tf("link inválido no pacote: %s", zf.Name))
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	_ = os.Remove(p)
	return os.Symlink(target, p)
}
