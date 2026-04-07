package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GlobTool finds files matching a glob pattern.
type GlobTool struct{}

func (t *GlobTool) Name() string        { return "glob" }
func (t *GlobTool) Cacheable() bool      { return true }
func (t *GlobTool) Description() string {
	return "Find files matching a glob pattern (e.g. '*.py', '**/*.ts'). " +
		"Results sorted by modification time (most recent first)."
}

func (t *GlobTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "The glob pattern to match files against.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "The directory to search in. Defaults to current working directory.",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *GlobTool) Execute(args map[string]any) string {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return "Error: pattern is required."
	}

	if strings.Contains(pattern, "../") || strings.Contains(pattern, "..\\") {
		return "Error: pattern must not contain directory traversal sequences."
	}

	base := "."
	if v, ok := args["path"].(string); ok && v != "" {
		base = v
	}

	info, err := os.Stat(base)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if !info.IsDir() {
		return fmt.Sprintf("Error: not a directory: %s", base)
	}

	fullPattern := filepath.Join(base, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return fmt.Sprintf("Error: invalid glob pattern: %v", err)
	}

	// Also try doublestar-like matching for ** patterns
	if strings.Contains(pattern, "**") {
		matches = walkGlob(base, pattern)
	}

	if len(matches) == 0 {
		return "No files found."
	}

	// Sort by modification time (most recent first)
	type fileEntry struct {
		path    string
		modTime int64
	}
	entries := make([]fileEntry, 0, len(matches))
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			entries = append(entries, fileEntry{path: m, modTime: 0})
			continue
		}
		entries = append(entries, fileEntry{path: m, modTime: info.ModTime().UnixNano()})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].modTime > entries[j].modTime
	})

	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(e.path)
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}

const maxWalkDepth = 50

// walkGlob implements recursive glob matching for ** patterns with depth limit.
func walkGlob(base, pattern string) []string {
	var matches []string
	baseDepth := strings.Count(filepath.Clean(base), string(filepath.Separator))

	_ = filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		depth := strings.Count(filepath.Clean(path), string(filepath.Separator)) - baseDepth
		if depth > maxWalkDepth {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return nil
		}
		if matchDoublestar(pattern, rel) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches
}

// matchDoublestar implements basic ** glob matching.
func matchDoublestar(pattern, name string) bool {
	// Handle **/ prefix: match any directory depth
	if strings.HasPrefix(pattern, "**/") {
		suffix := pattern[3:]
		// Try matching at every path level
		parts := strings.Split(name, string(filepath.Separator))
		for i := range parts {
			sub := strings.Join(parts[i:], string(filepath.Separator))
			matched, _ := filepath.Match(suffix, sub)
			if matched {
				return true
			}
		}
		return false
	}
	matched, _ := filepath.Match(pattern, name)
	return matched
}
