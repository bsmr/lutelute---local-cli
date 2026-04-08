package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var blockedPrefixes = []string{
	"/etc/", "/boot/", "/sbin/", "/usr/sbin/",
	"/dev/", "/proc/", "/sys/",
}

// WriteTool creates or overwrites files.
type WriteTool struct{}

func (t *WriteTool) Name() string        { return "write" }
func (t *WriteTool) Cacheable() bool      { return false }
func (t *WriteTool) Description() string {
	return "Write content to a file. Creates parent directories if needed. " +
		"Overwrites existing files. Blocks writes to sensitive system paths."
}

func (t *WriteTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The path to the file to write.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The content to write to the file.",
			},
		},
		"required": []string{"file_path", "content"},
	}
}

func (t *WriteTool) Execute(args map[string]any) string {
	filePath, _ := args["file_path"].(string)
	if filePath == "" {
		return "Error: file_path is required."
	}
	content, _ := args["content"].(string)

	if !isPathSafe(filePath) {
		return fmt.Sprintf("Error: write to %s blocked by security policy.", filePath)
	}

	info, err := os.Stat(filePath)
	if err == nil && info.IsDir() {
		return fmt.Sprintf("Error: path is a directory: %s", filePath)
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Sprintf("Error creating directories: %v", err)
	}

	mode := fileMode(filePath)
	if err := os.WriteFile(filePath, []byte(content), mode); err != nil {
		return fmt.Sprintf("Error writing file: %v", err)
	}

	lines := strings.Count(content, "\n")
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		lines++
	}
	return fmt.Sprintf("Successfully wrote %d bytes (%d lines) to %s", len(content), lines, filePath)
}

// sensitiveNames lists filename patterns that should be owner-only (0600).
var sensitiveNames = []string{
	".env", ".secret", "credentials", "id_rsa", "id_ed25519",
	".netrc", ".pgpass", ".my.cnf",
}

// fileMode returns 0600 for sensitive filenames, 0644 otherwise.
func fileMode(filePath string) os.FileMode {
	base := strings.ToLower(filepath.Base(filePath))
	for _, s := range sensitiveNames {
		if base == s || strings.HasPrefix(base, s+".") || strings.HasSuffix(base, s) {
			return 0o600
		}
	}
	return 0o644
}

func isPathSafe(filePath string) bool {
	resolved, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}

	for _, prefix := range blockedPrefixes {
		if strings.HasPrefix(resolved, prefix) {
			return false
		}
	}

	// For relative paths, ensure we stay within cwd
	if !filepath.IsAbs(filePath) {
		cwd, err := os.Getwd()
		if err != nil {
			return false
		}
		if !strings.HasPrefix(resolved, cwd) {
			return false
		}
	}
	return true
}
