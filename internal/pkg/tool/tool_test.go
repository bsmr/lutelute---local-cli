package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "line one\nline two\nline three\n"
	os.WriteFile(path, []byte(content), 0o644)

	rt := &ReadTool{}

	// Full read
	result := rt.Execute(map[string]any{"file_path": path})
	if !strings.Contains(result, "line one") || !strings.Contains(result, "line three") {
		t.Errorf("full read missing content: %s", result)
	}

	// Offset and limit
	result = rt.Execute(map[string]any{"file_path": path, "offset": float64(2), "limit": float64(1)})
	if !strings.Contains(result, "line two") {
		t.Errorf("offset read should contain 'line two': %s", result)
	}
	if strings.Contains(result, "line one") || strings.Contains(result, "line three") {
		t.Errorf("offset read should not contain other lines: %s", result)
	}

	// Missing file
	result = rt.Execute(map[string]any{"file_path": "/nonexistent/file"})
	if !strings.HasPrefix(result, "Error:") {
		t.Errorf("missing file should return error: %s", result)
	}
}

func TestWriteTool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "new.txt")

	wt := &WriteTool{}
	result := wt.Execute(map[string]any{"file_path": path, "content": "hello world\n"})
	if !strings.Contains(result, "Successfully") {
		t.Errorf("write should succeed: %s", result)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world\n" {
		t.Errorf("file content = %q", string(data))
	}
}

func TestWriteToolSensitivePermissions(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		wantMode os.FileMode
	}{
		{".env", 0o600},
		{".env.local", 0o600},
		{"credentials", 0o600},
		{"id_rsa", 0o600},
		{"id_ed25519", 0o600},
		{".netrc", 0o600},
		{"normal.txt", 0o644},
		{"main.go", 0o644},
	}

	wt := &WriteTool{}
	for _, tt := range tests {
		path := filepath.Join(dir, tt.name)
		wt.Execute(map[string]any{"file_path": path, "content": "test\n"})
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		got := info.Mode().Perm()
		if got != tt.wantMode {
			t.Errorf("%s: mode = %o, want %o", tt.name, got, tt.wantMode)
		}
	}
}

func TestEditTool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	os.WriteFile(path, []byte("foo bar baz\nfoo qux\n"), 0o644)

	et := &EditTool{}

	// Replace first occurrence
	result := et.Execute(map[string]any{
		"file_path": path,
		"old_text":  "foo",
		"new_text":  "FOO",
	})
	if !strings.Contains(result, "1 occurrence") {
		t.Errorf("should replace 1 occurrence: %s", result)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "FOO bar baz\nfoo qux\n" {
		t.Errorf("content = %q", string(data))
	}

	// Replace all
	os.WriteFile(path, []byte("aaa bbb aaa\n"), 0o644)
	result = et.Execute(map[string]any{
		"file_path":   path,
		"old_text":    "aaa",
		"new_text":    "ccc",
		"replace_all": true,
	})
	if !strings.Contains(result, "2 occurrence") {
		t.Errorf("should replace 2 occurrences: %s", result)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "ccc bbb ccc\n" {
		t.Errorf("content = %q", string(data))
	}
}

func TestGlobTool(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("go"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("go"), 0o644)
	os.WriteFile(filepath.Join(dir, "c.txt"), []byte("txt"), 0o644)

	gt := &GlobTool{}
	result := gt.Execute(map[string]any{"pattern": "*.go", "path": dir})
	if !strings.Contains(result, "a.go") || !strings.Contains(result, "b.go") {
		t.Errorf("should find .go files: %s", result)
	}
	if strings.Contains(result, "c.txt") {
		t.Errorf("should not find .txt files: %s", result)
	}
}

func TestGrepTool(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("func main() {\n\tfmt.Println(\"hello\")\n}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "other.txt"), []byte("no match here\n"), 0o644)

	gt := &GrepTool{}
	result := gt.Execute(map[string]any{"pattern": "func main", "path": dir})
	if !strings.Contains(result, "test.go:1:func main") {
		t.Errorf("should find 'func main': %s", result)
	}
}

func TestBashTool(t *testing.T) {
	bt := &BashTool{}
	result := bt.Execute(map[string]any{"command": "echo hello"})
	if result != "hello" {
		t.Errorf("bash echo = %q, want %q", result, "hello")
	}

	// Dangerous command
	result = bt.Execute(map[string]any{"command": "rm -rf /"})
	if !strings.Contains(result, "blocked") {
		t.Errorf("dangerous command should be blocked: %s", result)
	}
}

func TestToOllamaTool(t *testing.T) {
	bt := &BashTool{}
	def := ToOllamaTool(bt)
	if def.Type != "function" {
		t.Errorf("Type = %q", def.Type)
	}
	if def.Function.Name != "bash" {
		t.Errorf("Name = %q", def.Function.Name)
	}
}

func TestToolMap(t *testing.T) {
	tools := DefaultTools()
	m := ToolMap(tools)
	if _, ok := m["bash"]; !ok {
		t.Error("ToolMap should contain 'bash'")
	}
	if _, ok := m["read"]; !ok {
		t.Error("ToolMap should contain 'read'")
	}
	if len(m) != len(tools) {
		t.Errorf("ToolMap size = %d, want %d", len(m), len(tools))
	}
}
