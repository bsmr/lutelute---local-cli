// Package agent implements the LLM agent loop with tool execution.
package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.muehmer.eu/lai/internal/provider"
	"go.muehmer.eu/lai/internal/spinner"
	"go.muehmer.eu/lai/internal/token"
	"go.muehmer.eu/lai/internal/tool"
)

const (
	maxDisplayResult       = 200
	compactMessageThresh   = 50
	charsPerToken          = 4
	compactTokenThresh     = 24_000
	compactToolResultMax   = 200
	compactAssistantMax    = 500
	compactKeepRecent      = 10
)

// Loop runs the main agent loop: send messages to LLM, execute tool calls, repeat.
func Loop(
	prov provider.Provider,
	model string,
	tools []tool.Tool,
	messages *[]provider.Message,
	tracker *token.Tracker,
	opts *provider.ChatOptions,
	limiter *RateLimiter,
) error {
	toolMap := tool.ToolMap(tools)
	toolDefs := tool.FormatTools(tools)

	if limiter != nil {
		limiter.ResetTurn()
	}

	for {
		// Compact messages if needed
		if needsCompaction(*messages, opts) {
			compactMessages(messages)
		}

		slog.Debug("sending messages", "count", len(*messages), "model", model)

		// Send to LLM with streaming
		s := spinner.New("Thinking")
		s.Start()

		chunks, errCh := prov.ChatStream(model, *messages, toolDefs, opts)

		response := collectStreamingResponse(chunks, errCh, s, tracker)
		s.Stop()

		if response.Message == nil {
			fmt.Fprintln(os.Stderr, "\n(empty response from model)")
			return nil
		}

		// Append assistant response to conversation
		*messages = append(*messages, *response.Message)

		// No tool calls → done
		if len(response.Message.ToolCalls) == 0 {
			return nil
		}

		// Execute each tool call
		for _, tc := range response.Message.ToolCalls {
			funcName := tc.Function.Name
			args := tc.Function.Arguments

			// Handle arguments that might be JSON strings
			if argStr, ok := args[""].(string); ok && len(args) == 1 {
				var parsed map[string]any
				if json.Unmarshal([]byte(argStr), &parsed) == nil {
					args = parsed
				}
			}

			// Rate limit check
			if limiter != nil {
				if err := limiter.Check(funcName); err != nil {
					slog.Error("rate limit hit", "tool", funcName, "err", err)
					*messages = append(*messages, provider.Message{
						Role:    "tool",
						Content: fmt.Sprintf("Error: %v", err),
					})
					return nil
				}
			}

			t, exists := toolMap[funcName]
			if !exists {
				result := fmt.Sprintf("Error: unknown tool '%s'", funcName)
				*messages = append(*messages, provider.Message{
					Role:    "tool",
					Content: result,
				})
				continue
			}

			slog.Debug("tool call", "tool", funcName, "args", args)

			ts := spinner.New(fmt.Sprintf("Running %s", funcName))
			ts.Start()
			start := time.Now()
			result := t.Execute(args)
			elapsed := time.Since(start)
			ts.Stop()

			if limiter != nil {
				limiter.Record(funcName)
			}

			slog.Info("audit",
				slog.Group("event",
					slog.String("type", "tool_exec"),
					slog.String("tool", funcName),
					slog.Int("result_bytes", len(result)),
					slog.Duration("duration", elapsed),
				),
			)

			// Display truncated result
			display := result
			if len(display) > maxDisplayResult {
				display = display[:maxDisplayResult] + "..."
			}
			fmt.Fprintf(os.Stderr, "  → %s: %s\n", funcName, display)

			*messages = append(*messages, provider.Message{
				Role:    "tool",
				Content: result,
			})
		}
	}
}

// collectStreamingResponse reads chunks from the channel and prints to stdout.
func collectStreamingResponse(
	chunks <-chan provider.ChatChunk,
	errCh <-chan error,
	s *spinner.Spinner,
	tracker *token.Tracker,
) provider.ChatChunk {
	var contentParts []string
	var toolCalls []provider.ToolCall
	var lastChunk provider.ChatChunk
	spinnerStopped := false

	for chunk := range chunks {
		lastChunk = chunk

		if chunk.Message != nil {
			// Stop spinner on first content
			if !spinnerStopped && (chunk.Message.Content != "" || len(chunk.Message.ToolCalls) > 0) {
				s.Stop()
				spinnerStopped = true
			}

			// Stream content to stdout
			if chunk.Message.Content != "" {
				contentParts = append(contentParts, chunk.Message.Content)
				fmt.Print(chunk.Message.Content)
			}

			// Accumulate tool calls
			if len(chunk.Message.ToolCalls) > 0 {
				toolCalls = append(toolCalls, chunk.Message.ToolCalls...)
			}
		}

		// Print thinking content
		if chunk.Thinking != "" {
			if !spinnerStopped {
				s.Stop()
				spinnerStopped = true
			}
			fmt.Fprintf(os.Stderr, "> %s", chunk.Thinking)
		}
	}

	// Check for stream errors
	select {
	case err := <-errCh:
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nStream error: %v\n", err)
		}
	default:
	}

	// Ensure newline after content
	if len(contentParts) > 0 {
		fmt.Println()
	}

	// Record token usage
	if tracker != nil {
		tracker.Record(token.Usage{
			InputTokens:  lastChunk.PromptEvalCount,
			OutputTokens: lastChunk.EvalCount,
			Provider:     "ollama",
		})
	}

	// Build final response
	result := lastChunk
	result.Message = &provider.Message{
		Role:      "assistant",
		Content:   strings.Join(contentParts, ""),
		ToolCalls: toolCalls,
	}
	return result
}

// EstimateTokens estimates the token count of a message list.
func EstimateTokens(messages []provider.Message) int {
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)
		for _, tc := range msg.ToolCalls {
			argsJSON, _ := json.Marshal(tc.Function.Arguments)
			totalChars += len(argsJSON)
		}
	}
	return totalChars / charsPerToken
}

func needsCompaction(messages []provider.Message, opts *provider.ChatOptions) bool {
	if len(messages) > compactMessageThresh {
		return true
	}
	threshold := compactTokenThresh
	if opts != nil && opts.NumCtx > 0 {
		threshold = opts.NumCtx * 3 / 4
	}
	return EstimateTokens(messages) > threshold
}

func compactMessages(messages *[]provider.Message) {
	msgs := *messages

	// Find system messages at start
	systemEnd := 0
	for systemEnd < len(msgs) && msgs[systemEnd].Role == "system" {
		systemEnd++
	}

	// Keep recent messages
	if len(msgs)-systemEnd <= compactKeepRecent {
		return // not enough old messages to compact
	}

	compactEnd := len(msgs) - compactKeepRecent
	if compactEnd <= systemEnd {
		return
	}

	slog.Debug("compacting messages",
		"range_start", systemEnd,
		"range_end", compactEnd,
		"keep_recent", compactKeepRecent,
	)

	for i := systemEnd; i < compactEnd; i++ {
		msgs[i] = compactMessage(msgs[i])
	}
}

func compactMessage(msg provider.Message) provider.Message {
	switch msg.Role {
	case "system":
		return msg
	case "tool":
		if len(msg.Content) > compactToolResultMax {
			msg.Content = msg.Content[:compactToolResultMax] + "... [truncated for context]"
		}
	case "assistant":
		if len(msg.Content) > compactAssistantMax {
			msg.Content = msg.Content[:compactAssistantMax] + "... [truncated for context]"
		}
		msg.ToolCalls = nil
	}
	return msg
}
