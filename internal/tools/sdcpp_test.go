package tools

import (
	"context"
	"os"
	"testing"
)

func TestIsAPE(t *testing.T) {
	if bin, _ := Find(Diffusion, ""); bin != "" && !isAPE(bin) {
		t.Fatal("diffusionfile should be detected as APE")
	}
	if isAPE("/bin/sh") {
		t.Fatal("/bin/sh is not an APE")
	}
}

// SDCPP_ZIP=sd-…-vulkan.zip go test -run TestSDCppInstall ./internal/tools
// (installs into the app's bin folder).
func TestSDCppInstall(t *testing.T) {
	zip := os.Getenv("SDCPP_ZIP")
	if zip == "" {
		t.Skip("set SDCPP_ZIP")
	}
	copyTo := zip + ".copy.zip"
	data, _ := os.ReadFile(zip)
	_ = os.WriteFile(copyTo, data, 0o644)
	if err := InstallSDCpp(copyTo, "test"); err != nil {
		t.Fatal(err)
	}
	bin, version := SDCppBinary()
	if bin == "" || version != "test" {
		t.Fatalf("binary %q version %q", bin, version)
	}
	backends, err := ProbeBackends(context.Background(), Diffusion, bin, "")
	t.Log(backends, err)
	if len(backends) == 0 {
		t.Fatal("no backends detected")
	}
}
