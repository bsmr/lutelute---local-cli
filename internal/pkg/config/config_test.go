package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := New(nil)
	if cfg.Model != DefaultModel {
		t.Errorf("Model = %q, want %q", cfg.Model, DefaultModel)
	}
	if cfg.OllamaHost != DefaultOllamaHost {
		t.Errorf("OllamaHost = %q, want %q", cfg.OllamaHost, DefaultOllamaHost)
	}
	if cfg.Provider != DefaultProvider {
		t.Errorf("Provider = %q, want %q", cfg.Provider, DefaultProvider)
	}
	if cfg.NumCtx != DefaultNumCtx {
		t.Errorf("NumCtx = %d, want %d", cfg.NumCtx, DefaultNumCtx)
	}
}

func TestCLIArgsOverride(t *testing.T) {
	model := "llama3:8b"
	debug := true
	args := &CLIArgs{
		Model: &model,
		Debug: &debug,
	}
	cfg := New(args)
	if cfg.Model != model {
		t.Errorf("Model = %q, want %q", cfg.Model, model)
	}
	if !cfg.Debug {
		t.Error("Debug should be true")
	}
}

func TestEnvVarOverride(t *testing.T) {
	t.Setenv("LOCAL_CLI_MODEL", "gemma2:9b")
	cfg := New(nil)
	if cfg.Model != "gemma2:9b" {
		t.Errorf("Model = %q, want %q", cfg.Model, "gemma2:9b")
	}
}

func TestCLIArgsTakePriority(t *testing.T) {
	t.Setenv("LOCAL_CLI_MODEL", "env-model")
	model := "cli-model"
	args := &CLIArgs{Model: &model}
	cfg := New(args)
	if cfg.Model != "cli-model" {
		t.Errorf("Model = %q, want %q", cfg.Model, "cli-model")
	}
}

func TestLoadConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := "model=test-model\ndebug=true\n# comment\n\nnum_ctx=4096\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	vals, err := loadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if vals["model"] != "test-model" {
		t.Errorf("model = %q", vals["model"])
	}
	if vals["debug"] != "true" {
		t.Errorf("debug = %q", vals["debug"])
	}
	if vals["num_ctx"] != "4096" {
		t.Errorf("num_ctx = %q", vals["num_ctx"])
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"1", true},
		{"yes", true},
		{"YES", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"", false},
	}
	for _, tt := range tests {
		got := parseBool(tt.input)
		if got != tt.want {
			t.Errorf("parseBool(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home dir")
	}
	got := expandHome("~/test")
	want := filepath.Join(home, "test")
	if got != want {
		t.Errorf("expandHome(~/test) = %q, want %q", got, want)
	}

	// Non-home path should be unchanged
	got = expandHome("/absolute/path")
	if got != "/absolute/path" {
		t.Errorf("expandHome(/absolute/path) = %q", got)
	}
}
