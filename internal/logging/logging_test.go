package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"", slog.LevelInfo},
		{"unknown", slog.LevelInfo},
	}
	for _, tt := range tests {
		got := ParseLevel(tt.input)
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSetupTerminalOnly(t *testing.T) {
	if err := Setup("info", ""); err != nil {
		t.Fatalf("Setup(info, \"\") failed: %v", err)
	}
	// Verify logger works
	slog.Info("test message", "key", "value")
}

func TestSetupWithFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	if err := Setup("debug", path); err != nil {
		t.Fatalf("Setup with file failed: %v", err)
	}

	slog.Info("test file log", "key", "value")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("log file should contain data")
	}
}

func TestSetupInvalidFile(t *testing.T) {
	err := Setup("info", "/nonexistent/dir/file.log")
	if err == nil {
		t.Error("should fail for invalid path")
	}
}
