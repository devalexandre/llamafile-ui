package tools

import (
	"reflect"
	"testing"
)

func TestParseBackends(t *testing.T) {
	whisper := `register_backend: registered backend CUDA (1 devices)
register_backend: registered backend CPU (1 devices)
whisper_backend_init_gpu: using CUDA0 backend`
	if got := parseBackends(Whisper, whisper); !reflect.DeepEqual(got, []string{"CUDA", "CPU"}) {
		t.Fatalf("whisper: %v", got)
	}
	diffusion := `[DEBUG] ggml_extend_backend.cpp:311  - Found 1 backend devices:
[DEBUG] ggml_extend_backend.cpp:314  - #0: CPU
[DEBUG] ggml_extend_backend.cpp:291  - Initializing backend: CPU
[DEBUG] ggml_extend_backend.cpp:356  - Using CPU backend`
	if got := parseBackends(Diffusion, diffusion); !reflect.DeepEqual(got, []string{"CPU"}) {
		t.Fatalf("diffusion: %v", got)
	}
	sdcpp := `ggml_vulkan: 0 = NVIDIA GeForce RTX 2060 (NVIDIA) | uma: 0
load_backend: loaded Vulkan backend from /x/libggml-vulkan.so
load_backend: loaded CPU backend from /x/libggml-cpu-haswell.so`
	if got := parseBackends(Diffusion, sdcpp); !reflect.DeepEqual(got, []string{"Vulkan", "CPU"}) {
		t.Fatalf("sd.cpp: %v", got)
	}
	wcpp := `whisper_backend_init_gpu: device 0: Vulkan0 (type: 1)
whisper_backend_init_gpu: using Vulkan0 backend`
	if got := parseBackends(Whisper, wcpp); !reflect.DeepEqual(got, []string{"Vulkan"}) {
		t.Fatalf("whisper.cpp: %v", got)
	}
	if !Newer("0.10.7", "0.10.6") || Newer("0.10.6", "0.10.6") || !Newer("0.11.0", "0.10.9") {
		t.Fatal("Newer")
	}
}
