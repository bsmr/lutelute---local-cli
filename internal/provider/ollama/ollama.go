// Package ollama implements the Ollama HTTP client and LLM provider.
package ollama

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"go.muehmer.eu/lai/internal/provider"
	"go.muehmer.eu/lai/internal/security"
)

const (
	providerName   = "ollama"
	defaultTimeout = 30 * time.Second
	streamTimeout  = 600 * time.Second
)

// rawStreamChunk captures all fields from a streaming chunk in a single unmarshal.
type rawStreamChunk struct {
	Message *struct {
		Role      string              `json:"role"`
		Content   string              `json:"content"`
		ToolCalls []provider.ToolCall `json:"tool_calls,omitempty"`
		Thinking  string              `json:"thinking,omitempty"`
	} `json:"message,omitempty"`
	Done            bool   `json:"done"`
	Error           string `json:"error,omitempty"`
	PromptEvalCount int    `json:"prompt_eval_count,omitempty"`
	EvalCount       int    `json:"eval_count,omitempty"`
}

// Client is an HTTP client for the Ollama REST API.
type Client struct {
	baseURL      string
	httpClient   *http.Client
	streamClient *http.Client
}

// NewClient creates a new Ollama HTTP client.
func NewClient(baseURL string) (*Client, error) {
	if !security.ValidateOllamaHost(baseURL) {
		return nil, fmt.Errorf("ollama host must be localhost: %s", baseURL)
	}
	return &Client{
		baseURL:      baseURL,
		httpClient:   &http.Client{Timeout: defaultTimeout},
		streamClient: &http.Client{Timeout: streamTimeout},
	}, nil
}

// GetVersion calls GET /api/version.
func (c *Client) GetVersion() (map[string]any, error) {
	return c.doJSON("GET", "/api/version", nil)
}

// ListModels calls GET /api/tags and returns the models list.
func (c *Client) ListModels() ([]provider.ModelInfo, error) {
	resp, err := c.doJSON("GET", "/api/tags", nil)
	if err != nil {
		return nil, err
	}
	raw, ok := resp["models"]
	if !ok {
		return nil, nil
	}
	modelsJSON, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var models []provider.ModelInfo
	if err := json.Unmarshal(modelsJSON, &models); err != nil {
		return nil, err
	}
	return models, nil
}

// ShowModel calls POST /api/show for model metadata.
func (c *Client) ShowModel(model string) (map[string]any, error) {
	return c.doJSON("POST", "/api/show", map[string]any{"name": model})
}

// ListRunningModels calls GET /api/ps.
func (c *Client) ListRunningModels() ([]map[string]any, error) {
	resp, err := c.doJSON("GET", "/api/ps", nil)
	if err != nil {
		return nil, err
	}
	raw, ok := resp["models"]
	if !ok {
		return nil, nil
	}
	modelsJSON, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var models []map[string]any
	err = json.Unmarshal(modelsJSON, &models)
	return models, err
}

// Embed calls POST /api/embed and returns embeddings.
func (c *Client) Embed(model string, input any) ([][]float64, error) {
	resp, err := c.doJSON("POST", "/api/embed", map[string]any{
		"model": model,
		"input": input,
	})
	if err != nil {
		return nil, err
	}
	raw, ok := resp["embeddings"]
	if !ok {
		return nil, fmt.Errorf("no embeddings in response")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var embeddings [][]float64
	err = json.Unmarshal(data, &embeddings)
	return embeddings, err
}

// DeleteModel calls DELETE /api/delete.
func (c *Client) DeleteModel(model string) error {
	return c.doNoContent("DELETE", "/api/delete", map[string]any{"name": model})
}

// ChatStream sends a streaming chat request and returns chunks via channel.
func (c *Client) ChatStream(model string, messages []provider.Message, tools []provider.ToolDefinition, opts *provider.ChatOptions) (<-chan provider.ChatChunk, <-chan error) {
	chunks := make(chan provider.ChatChunk, 16)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errCh)

		body := buildChatBody(model, messages, tools, opts, true)
		resp, err := c.doStreamRequest("/api/chat", body)
		if err != nil {
			errCh <- &provider.ConnectionError{Provider: providerName, Cause: err}
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}

			var raw rawStreamChunk
			if err := json.Unmarshal(line, &raw); err != nil {
				continue // skip malformed JSON
			}

			if raw.Error != "" {
				errCh <- &provider.StreamError{Provider: providerName, Cause: fmt.Errorf("%s", raw.Error)}
				return
			}

			chunk := provider.ChatChunk{
				Done:            raw.Done,
				PromptEvalCount: raw.PromptEvalCount,
				EvalCount:       raw.EvalCount,
			}
			if raw.Message != nil {
				chunk.Message = &provider.Message{
					Role:      raw.Message.Role,
					Content:   raw.Message.Content,
					ToolCalls: raw.Message.ToolCalls,
				}
				chunk.Thinking = raw.Message.Thinking
			}

			chunks <- chunk
		}
		if err := scanner.Err(); err != nil {
			errCh <- &provider.StreamError{Provider: providerName, Cause: err}
		}
	}()

	return chunks, errCh
}

// Chat sends a non-streaming chat request.
func (c *Client) Chat(model string, messages []provider.Message, tools []provider.ToolDefinition, opts *provider.ChatOptions) (*provider.ChatChunk, error) {
	body := buildChatBody(model, messages, tools, opts, false)
	resp, err := c.doJSON("POST", "/api/chat", body)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var chunk provider.ChatChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil, err
	}
	return &chunk, nil
}

func buildChatBody(model string, messages []provider.Message, tools []provider.ToolDefinition, opts *provider.ChatOptions, stream bool) map[string]any {
	body := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   stream,
	}

	if len(tools) > 0 {
		body["tools"] = tools
	}

	options := make(map[string]any)
	if opts != nil {
		if opts.NumCtx > 0 {
			options["num_ctx"] = opts.NumCtx
		}
		if opts.Temperature != nil {
			options["temperature"] = *opts.Temperature
		}
		if opts.TopP != nil {
			options["top_p"] = *opts.TopP
		}
		if opts.TopK != nil {
			options["top_k"] = *opts.TopK
		}
		if opts.Think != nil {
			body["think"] = *opts.Think
		}
		if opts.KeepAlive != nil {
			body["keep_alive"] = opts.KeepAlive
		}
		if opts.Format != nil {
			body["format"] = opts.Format
		}
	}
	body["options"] = options

	return body
}

// doJSON executes an HTTP request and returns parsed JSON.
func (c *Client) doJSON(method, path string, data any) (map[string]any, error) {
	var bodyReader io.Reader
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	slog.Debug("ollama request", "method", method, "path", path)

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, &provider.ConnectionError{Provider: providerName, Cause: err}
	}
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &provider.ConnectionError{Provider: providerName, Cause: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &provider.ConnectionError{Provider: providerName, Cause: err}
	}

	if resp.StatusCode >= 400 {
		slog.Error("ollama request failed", "status", resp.StatusCode, "path", path)
		return nil, &provider.RequestError{
			Provider:   "ollama",
			StatusCode: resp.StatusCode,
			Body:       truncateBody(string(body)),
		}
	}

	var result map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("JSON decode error: %w", err)
		}
	}
	return result, nil
}

func (c *Client) doNoContent(method, path string, data any) error {
	var bodyReader io.Reader
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return &provider.ConnectionError{Provider: providerName, Cause: err}
	}
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &provider.ConnectionError{Provider: providerName, Cause: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return &provider.RequestError{
			Provider:   "ollama",
			StatusCode: resp.StatusCode,
			Body:       truncateBody(string(body)),
		}
	}
	return nil
}

func (c *Client) doStreamRequest(path string, data any) (*http.Response, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.streamClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &provider.RequestError{
			Provider:   "ollama",
			StatusCode: resp.StatusCode,
			Body:       truncateBody(string(body)),
		}
	}

	return resp, nil
}

const maxErrorBodyLen = 500

func truncateBody(s string) string {
	if len(s) > maxErrorBodyLen {
		return s[:maxErrorBodyLen] + "... [truncated]"
	}
	return s
}

// Provider wraps Client to implement the provider.Provider interface.
type Provider struct {
	client *Client
}

// NewProvider creates an Ollama provider.
func NewProvider(baseURL string) (*Provider, error) {
	c, err := NewClient(baseURL)
	if err != nil {
		return nil, err
	}
	return &Provider{client: c}, nil
}

func (p *Provider) Name() string { return "ollama" }

func (p *Provider) Chat(model string, messages []provider.Message, tools []provider.ToolDefinition, opts *provider.ChatOptions) (*provider.ChatChunk, error) {
	return p.client.Chat(model, messages, tools, opts)
}

func (p *Provider) ChatStream(model string, messages []provider.Message, tools []provider.ToolDefinition, opts *provider.ChatOptions) (<-chan provider.ChatChunk, <-chan error) {
	return p.client.ChatStream(model, messages, tools, opts)
}

func (p *Provider) ListModels() ([]provider.ModelInfo, error) {
	return p.client.ListModels()
}

func (p *Provider) GetModelInfo(model string) (map[string]any, error) {
	return p.client.ShowModel(model)
}

// Client returns the underlying Ollama HTTP client.
func (p *Provider) OllamaClient() *Client {
	return p.client
}
