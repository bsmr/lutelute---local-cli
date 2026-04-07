package tool

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

// ReadTool reads file contents with optional offset and limit.
type ReadTool struct{}

func (t *ReadTool) Name() string        { return "read" }
func (t *ReadTool) Cacheable() bool      { return true }
func (t *ReadTool) Description() string {
	return "Read the contents of a file. Returns numbered lines. " +
		"Use offset and limit to read specific sections of large files."
}

func (t *ReadTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The path to the file to read.",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Line number to start reading from (1-based). Defaults to 1.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of lines to read. Defaults to reading the entire file.",
			},
		},
		"required": []string{"file_path"},
	}
}

func (t *ReadTool) Execute(args map[string]any) string {
	filePath, _ := args["file_path"].(string)
	if filePath == "" {
		return "Error: file_path is required."
	}

	offset := 1
	if v, ok := args["offset"]; ok {
		if n, ok := toInt(v); ok && n >= 1 {
			offset = n
		}
	}
	limit := 0 // 0 means no limit
	if v, ok := args["limit"]; ok {
		if n, ok := toInt(v); ok && n >= 1 {
			limit = n
		}
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Sprintf("Error: not a regular file: %s", filePath)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	defer f.Close()

	// Binary detection: read first 8192 bytes and check for null byte
	header := make([]byte, 8192)
	n, _ := f.Read(header)
	if bytes.ContainsRune(header[:n], 0) {
		return fmt.Sprintf("Error: binary file detected: %s", filePath)
	}
	// Seek back to start
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	const maxReadBytes = 5 * 1024 * 1024 // 5 MB output limit

	var sb strings.Builder
	lineNum := 0
	count := 0

	for scanner.Scan() {
		lineNum++
		if lineNum < offset {
			continue
		}
		if limit > 0 && count >= limit {
			break
		}
		fmt.Fprintf(&sb, "%6d\t%s\n", lineNum, scanner.Text())
		count++
		if sb.Len() > maxReadBytes {
			fmt.Fprintf(&sb, "\n... [output truncated at 5 MB]")
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Sprintf("Error reading file: %v", err)
	}

	if sb.Len() == 0 {
		return "(empty file)"
	}
	return sb.String()
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}
