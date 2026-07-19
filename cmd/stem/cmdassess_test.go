package main

import "testing"

const testGiB = uint64(1) << 30

func TestAssessFitVerdict(t *testing.T) {
	cases := []struct {
		name       string
		usableVRAM uint64
		required   uint64
		ram        uint64
		want       string
	}{
		{"comfortable fit in vram", 23 * testGiB, 10 * testGiB, 32 * testGiB, verdictFitsFully},
		{"exactly at headroom boundary", 10 * testGiB, 9 * testGiB, 32 * testGiB, verdictFitsFully},
		{"just past headroom boundary", 10 * testGiB, 9*testGiB + 1, 32 * testGiB, verdictFitsTight},
		{"exactly fills usable vram", 10 * testGiB, 10 * testGiB, 32 * testGiB, verdictFitsTight},
		{"spills into ram", 10 * testGiB, 20 * testGiB, 32 * testGiB, verdictGPUCPUSplit},
		{"exactly fills vram plus ram", 10 * testGiB, 42 * testGiB, 32 * testGiB, verdictGPUCPUSplit},
		{"too big even with ram", 10 * testGiB, 42*testGiB + 1, 32 * testGiB, verdictExceedsMachine},
		{"no gpu fits in ram", 0, 8 * testGiB, 32 * testGiB, verdictCPUOnly},
		{"no gpu exactly fills ram", 0, 32 * testGiB, 32 * testGiB, verdictCPUOnly},
		{"no gpu too big for ram", 0, 33 * testGiB, 32 * testGiB, verdictExceedsMachine},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := assessFitVerdict(tc.usableVRAM, tc.required, tc.ram)
			if got != tc.want {
				t.Errorf("assessFitVerdict(%d, %d, %d) = %q, want %q",
					tc.usableVRAM, tc.required, tc.ram, got, tc.want)
			}
		})
	}
}

func TestAssessUsableVRAM(t *testing.T) {
	if got := assessUsableVRAM(0); got != 0 {
		t.Errorf("no GPU should yield 0 usable VRAM, got %d", got)
	}
	if got := assessUsableVRAM(assessVRAMReserveBytes); got != 0 {
		t.Errorf("VRAM equal to the reserve should yield 0, got %d", got)
	}
	if got := assessUsableVRAM(24 * testGiB); got != 23*testGiB {
		t.Errorf("24 GiB VRAM should yield 23 GiB usable, got %d", got)
	}
}

func TestAssessRequiredBytes(t *testing.T) {
	size := 5 * testGiB // roughly an 8B model at Q4

	zeroCtx := assessRequiredBytes(size, 0)
	if zeroCtx != size {
		t.Errorf("zero context should add no KV cache: got %d, want %d", zeroCtx, size)
	}

	small := assessRequiredBytes(size, 2048)
	large := assessRequiredBytes(size, 8192)
	if small <= size || large <= small {
		t.Errorf("required bytes must grow with context: size=%d ctx2048=%d ctx8192=%d", size, small, large)
	}
	// ~8B params * 16 KiB/token * 8192 tokens is on the order of 1 GiB; the
	// estimate should stay in a sane band, not be off by orders of magnitude.
	kv := large - size
	if kv < testGiB/2 || kv > 4*testGiB {
		t.Errorf("KV estimate out of expected band: %d bytes", kv)
	}

	if got := assessRequiredBytes(size, -100); got != size {
		t.Errorf("negative context must clamp to zero KV: got %d", got)
	}
}

func TestParseNvidiaSMIMemory(t *testing.T) {
	t.Run("single gpu", func(t *testing.T) {
		gpus, err := parseNvidiaSMIMemory("24576, 23001\n")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(gpus) != 1 {
			t.Fatalf("expected 1 GPU, got %d", len(gpus))
		}
		if gpus[0].TotalBytes != 24576<<20 || gpus[0].FreeBytes != 23001<<20 {
			t.Errorf("unexpected GPU memory: %+v", gpus[0])
		}
	})

	t.Run("multi gpu pools independently", func(t *testing.T) {
		gpus, err := parseNvidiaSMIMemory("24576, 20000\n11264, 11000\n")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(gpus) != 2 {
			t.Fatalf("expected 2 GPUs, got %d", len(gpus))
		}
		if gpus[1].TotalBytes != 11264<<20 {
			t.Errorf("second GPU total = %d, want %d", gpus[1].TotalBytes, uint64(11264)<<20)
		}
	})

	t.Run("empty output yields no gpus", func(t *testing.T) {
		gpus, err := parseNvidiaSMIMemory("\n")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(gpus) != 0 {
			t.Errorf("expected no GPUs, got %d", len(gpus))
		}
	})

	t.Run("garbage output errors", func(t *testing.T) {
		if _, err := parseNvidiaSMIMemory("NVIDIA-SMI has failed\n"); err == nil {
			t.Error("expected error for malformed output")
		}
		if _, err := parseNvidiaSMIMemory("abc, def\n"); err == nil {
			t.Error("expected error for non-numeric fields")
		}
	})
}

func TestParseMemAvailableBytes(t *testing.T) {
	meminfo := "MemTotal:       65536000 kB\nMemFree:         1234567 kB\nMemAvailable:   32768000 kB\n"
	got, err := parseMemAvailableBytes(meminfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := uint64(32768000) * 1024; got != want {
		t.Errorf("parseMemAvailableBytes = %d, want %d", got, want)
	}

	if _, err := parseMemAvailableBytes("MemTotal: 1 kB\n"); err == nil {
		t.Error("expected error when MemAvailable is missing")
	}
}

func TestParseOllamaTags(t *testing.T) {
	body := `{"models":[{"name":"llama3.2:latest","size":2019393189},{"name":"qwen2.5-coder:14b","size":8988124315}]}`
	models, err := parseOllamaTags([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].Name != "llama3.2:latest" || models[0].SizeBytes != 2019393189 {
		t.Errorf("unexpected first model: %+v", models[0])
	}

	if _, err := parseOllamaTags([]byte("not json")); err == nil {
		t.Error("expected error for malformed body")
	}
}

func TestBuildAssessReport(t *testing.T) {
	hw := assessHardware{
		GPUs:              []assessGPU{{TotalBytes: 24 * testGiB, FreeBytes: 20 * testGiB}},
		VRAMTotalBytes:    24 * testGiB,
		CPUCores:          16,
		RAMAvailableBytes: 32 * testGiB,
	}
	models := []assessModel{
		{Name: "small", SizeBytes: 2 * testGiB},
		{Name: "huge", SizeBytes: 200 * testGiB},
	}

	report := buildAssessReport(hw, models, 8192)
	if len(report.Models) != 2 {
		t.Fatalf("expected 2 model fits, got %d", len(report.Models))
	}
	if report.Models[0].Verdict != verdictFitsFully {
		t.Errorf("small model verdict = %q, want %q", report.Models[0].Verdict, verdictFitsFully)
	}
	if report.Models[1].Verdict != verdictExceedsMachine {
		t.Errorf("huge model verdict = %q, want %q", report.Models[1].Verdict, verdictExceedsMachine)
	}
	if report.Models[0].RequiredBytes <= report.Models[0].SizeBytes {
		t.Errorf("required bytes should include KV overhead beyond model size")
	}

	// No GPU and no models: the report must still be well-formed (cpu-only path).
	empty := buildAssessReport(assessHardware{CPUCores: 4, RAMAvailableBytes: 8 * testGiB}, nil, 8192)
	if empty.Models == nil || len(empty.Models) != 0 {
		t.Errorf("expected empty non-nil model list, got %#v", empty.Models)
	}
}
