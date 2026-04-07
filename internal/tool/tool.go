// Package tool defines the tool interface and tool execution utilities.
package tool

import "go.muehmer.eu/lai/internal/provider"

// Tool is the interface that all agent tools must implement.
type Tool interface {
	// Name returns the tool identifier used in function calls.
	Name() string

	// Description returns a human-readable description for the LLM.
	Description() string

	// Parameters returns the JSON Schema for accepted parameters.
	Parameters() map[string]any

	// Execute runs the tool with the given arguments and returns a result string.
	Execute(args map[string]any) string

	// Cacheable returns true if tool results can be safely cached.
	Cacheable() bool
}

// ToOllamaTool converts a Tool to the Ollama/OpenAI tool definition format.
func ToOllamaTool(t Tool) provider.ToolDefinition {
	return provider.ToolDefinition{
		Type: "function",
		Function: provider.ToolFunction{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		},
	}
}

// FormatTools converts a slice of Tools to provider tool definitions.
func FormatTools(tools []Tool) []provider.ToolDefinition {
	defs := make([]provider.ToolDefinition, len(tools))
	for i, t := range tools {
		defs[i] = ToOllamaTool(t)
	}
	return defs
}

// ToolMap builds a name-to-tool lookup map.
func ToolMap(tools []Tool) map[string]Tool {
	m := make(map[string]Tool, len(tools))
	for _, t := range tools {
		m[t.Name()] = t
	}
	return m
}
