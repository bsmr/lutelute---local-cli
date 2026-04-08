// Package provider defines the LLM provider interface and shared types.
package provider

import (
	"errors"
	"fmt"
)

// Message role constants.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Message represents a chat message in the normalized format.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Function FunctionCall `json:"function"`
}

// FunctionCall holds the function name and arguments for a tool call.
type FunctionCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ChatChunk is a single chunk from a streaming chat response.
type ChatChunk struct {
	Message         *Message `json:"message,omitempty"`
	Done            bool     `json:"done"`
	Thinking        string   `json:"thinking,omitempty"`
	PromptEvalCount int      `json:"prompt_eval_count,omitempty"`
	EvalCount       int      `json:"eval_count,omitempty"`
}

// ToolDefinition is the Ollama/OpenAI tool format sent to the LLM.
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a callable function for the LLM.
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// Provider is the interface that all LLM backends must implement.
type Provider interface {
	// Name returns the provider identifier (e.g. "ollama", "claude").
	Name() string

	// Chat sends a non-streaming chat request.
	Chat(model string, messages []Message, tools []ToolDefinition, opts *ChatOptions) (*ChatChunk, error)

	// ChatStream sends a streaming chat request and returns chunks via channel.
	// The channel is closed when the stream ends. Errors are sent as the last
	// chunk with Done=true or via the error channel.
	ChatStream(model string, messages []Message, tools []ToolDefinition, opts *ChatOptions) (<-chan ChatChunk, <-chan error)

	// ListModels returns a list of available models.
	ListModels() ([]ModelInfo, error)

	// GetModelInfo returns details about a specific model.
	GetModelInfo(model string) (map[string]any, error)
}

// ChatOptions holds optional parameters for chat requests.
type ChatOptions struct {
	MaxTokens   int
	NumCtx      int
	Temperature *float64
	TopP        *float64
	TopK        *int
	Think       *bool
	KeepAlive   any
	Format      any
}

// ModelInfo holds basic model metadata.
type ModelInfo struct {
	Name       string `json:"name"`
	Size       int64  `json:"size,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
}

// Error types for provider-agnostic error handling.
var (
	ErrConnection = errors.New("provider connection error")
	ErrRequest    = errors.New("provider request error")
	ErrStream     = errors.New("provider stream error")
)

// ConnectionError wraps ErrConnection with details.
type ConnectionError struct {
	Provider string
	Cause    error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("%s: connection failed: %v", e.Provider, e.Cause)
}

func (e *ConnectionError) Unwrap() error { return ErrConnection }

// RequestError wraps ErrRequest with details.
type RequestError struct {
	Provider   string
	StatusCode int
	Body       string
}

func (e *RequestError) Error() string {
	return fmt.Sprintf("%s: request error (HTTP %d): %s", e.Provider, e.StatusCode, e.Body)
}

func (e *RequestError) Unwrap() error { return ErrRequest }

// StreamError wraps ErrStream with details.
type StreamError struct {
	Provider string
	Cause    error
}

func (e *StreamError) Error() string {
	return fmt.Sprintf("%s: stream error: %v", e.Provider, e.Cause)
}

func (e *StreamError) Unwrap() error { return ErrStream }
