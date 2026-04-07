package tool

import (
	"fmt"
	"os"
	"strings"
)

// EditTool performs text replacement in files.
type EditTool struct{}

func (t *EditTool) Name() string        { return "edit" }
func (t *EditTool) Cacheable() bool      { return false }
func (t *EditTool) Description() string {
	return "Replace text in a file. Finds exact matches of old_text and replaces with new_text. " +
		"Use replace_all=true to replace all occurrences."
}

func (t *EditTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The path to the file to edit.",
			},
			"old_text": map[string]any{
				"type":        "string",
				"description": "The exact text to find in the file.",
			},
			"new_text": map[string]any{
				"type":        "string",
				"description": "The text to replace old_text with.",
			},
			"replace_all": map[string]any{
				"type":        "boolean",
				"description": "If true, replace all occurrences. Defaults to false (first only).",
			},
		},
		"required": []string{"file_path", "old_text", "new_text"},
	}
}

func (t *EditTool) Execute(args map[string]any) string {
	filePath, _ := args["file_path"].(string)
	if filePath == "" {
		return "Error: file_path is required."
	}
	oldText, _ := args["old_text"].(string)
	if oldText == "" {
		return "Error: old_text is required and must be non-empty."
	}
	newText, _ := args["new_text"].(string)
	replaceAll, _ := args["replace_all"].(bool)

	if !isPathSafe(filePath) {
		return fmt.Sprintf("Error: edit to %s blocked by security policy.", filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	content := string(data)

	count := strings.Count(content, oldText)
	if count == 0 {
		return fmt.Sprintf("Error: old_text not found in %s", filePath)
	}

	var replaced string
	var occurrences int
	if replaceAll {
		replaced = strings.ReplaceAll(content, oldText, newText)
		occurrences = count
	} else {
		replaced = strings.Replace(content, oldText, newText, 1)
		occurrences = 1
	}

	if err := os.WriteFile(filePath, []byte(replaced), 0o644); err != nil {
		return fmt.Sprintf("Error writing file: %v", err)
	}

	return makeDiffOutput(filePath, oldText, newText, occurrences)
}

func makeDiffOutput(filePath, oldText, newText string, occurrences int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n", filePath)
	fmt.Fprintf(&sb, "+++ %s\n", filePath)
	fmt.Fprintf(&sb, "@@ replaced %d occurrence(s) @@\n", occurrences)
	for line := range strings.SplitSeq(oldText, "\n") {
		fmt.Fprintf(&sb, "-%s\n", line)
	}
	for line := range strings.SplitSeq(newText, "\n") {
		fmt.Fprintf(&sb, "+%s\n", line)
	}
	return sb.String()
}
