package cli

import "testing"

func TestParseBytes(t *testing.T) {
	tests := map[string]int64{
		"1":  1,
		"1k": 1024,
		"2M": 2 * 1024 * 1024,
		"3g": 3 * 1024 * 1024 * 1024,
	}
	for in, want := range tests {
		got, err := ParseBytes(in)
		if err != nil {
			t.Fatalf("ParseBytes(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("ParseBytes(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParse(t *testing.T) {
	cfg, err := Parse([]string{"--memory=64m", "--cpu", "0.5", "--env", "A=B", "echo", "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MemoryBytes != 64*1024*1024 || cfg.CPUQuota != 0.5 {
		t.Fatalf("unexpected limits: %+v", cfg)
	}
	if len(cfg.Command) != 2 || cfg.Command[0] != "echo" || cfg.Command[1] != "hi" {
		t.Fatalf("unexpected command: %+v", cfg.Command)
	}
}

func TestParseRequiresCommandAndLimit(t *testing.T) {
	if _, err := Parse([]string{"--memory=64m"}); err == nil {
		t.Fatal("expected missing command error")
	}
	if _, err := Parse([]string{"echo"}); err == nil {
		t.Fatal("expected missing limit error")
	}
}
