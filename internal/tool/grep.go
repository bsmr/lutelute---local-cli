package tool

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const maxGrepResults = 500

// GrepTool searches file contents with regex patterns.
type GrepTool struct{}

func (t *GrepTool) Name() string        { return "grep" }
func (t *GrepTool) Cacheable() bool      { return true }
func (t *GrepTool) Description() string {
	return "Search file contents using regular expressions. " +
		"Returns matching lines with file paths and line numbers."
}

func (t *GrepTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "The regular expression pattern to search for.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "File or directory to search in. Defaults to current directory.",
			},
			"include": map[string]any{
				"type":        "string",
				"description": "Glob pattern to filter files (e.g. '*.py'). Defaults to all files.",
			},
			"case_insensitive": map[string]any{
				"type":        "boolean",
				"description": "Case-insensitive matching. Defaults to false.",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *GrepTool) Execute(args map[string]any) string {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return "Error: pattern is required."
	}

	flags := ""
	if ci, _ := args["case_insensitive"].(bool); ci {
		flags = "(?i)"
	}
	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return fmt.Sprintf("Error: invalid regex: %v", err)
	}

	base := "."
	if v, ok := args["path"].(string); ok && v != "" {
		base = v
	}

	include, _ := args["include"].(string)

	info, err := os.Stat(base)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	var files []string
	if !info.IsDir() {
		files = []string{base}
	} else {
		files = collectFiles(base, include)
	}

	var sb strings.Builder
	count := 0

	for _, filePath := range files {
		if count >= maxGrepResults {
			fmt.Fprintf(&sb, "\n... [truncated at %d matches]", maxGrepResults)
			break
		}

		matches := searchFile(filePath, re)
		for _, m := range matches {
			if count >= maxGrepResults {
				break
			}
			sb.WriteString(m)
			sb.WriteByte('\n')
			count++
		}
	}

	if count == 0 {
		return "No matches found."
	}
	return strings.TrimRight(sb.String(), "\n")
}

func collectFiles(base, include string) []string {
	var files []string

	_ = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "__pycache__" || name == ".venv" {
				return filepath.SkipDir
			}
			return nil
		}
		if include != "" {
			matched, _ := filepath.Match(include, d.Name())
			if !matched {
				return nil
			}
		}
		files = append(files, path)
		return nil
	})

	sort.Strings(files)
	return files
}

func searchFile(filePath string, re *regexp.Regexp) []string {
	f, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	// Binary detection
	header := make([]byte, 8192)
	n, _ := f.Read(header)
	if bytes.ContainsRune(header[:n], 0) {
		return nil
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil
	}

	var matches []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			matches = append(matches, fmt.Sprintf("%s:%d:%s", filePath, lineNum, line))
		}
	}
	return matches
}
