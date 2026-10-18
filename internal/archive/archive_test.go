package archive

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// PIPER_TGZ=piper_linux_x86_64.tar.gz go test -run TestExtract ./internal/archive
func TestExtractRealArchive(t *testing.T) {
	archive := os.Getenv("PIPER_TGZ")
	if archive == "" {
		t.Skip("set PIPER_TGZ")
	}
	dest := t.TempDir()
	if err := extractTarGz(archive, dest); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(filepath.Join(dest, "piper", "piper"), "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}

func TestTargetRejectsTraversal(t *testing.T) {
	if _, err := target("/tmp/x", "../etc/passwd"); err == nil {
		t.Fatal("path traversal accepted")
	}
	if _, err := target("/tmp/x", "piper/piper"); err != nil {
		t.Fatal(err)
	}
}
