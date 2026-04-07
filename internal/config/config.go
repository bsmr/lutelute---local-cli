// Package config implements layered configuration: CLI > env > file > defaults.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.muehmer.eu/lai/internal/security"
)

// Defaults for all configuration keys.
const (
	DefaultModel          = "qwen3.5:9b-q4_K_M"
	DefaultOllamaHost     = "http://localhost:11434"
	DefaultProvider       = "ollama"
	DefaultStateDir       = "~/.local/state/local-cli"
	DefaultConfigFile     = "~/.config/local-cli/config"
	DefaultNumCtx         = 8192
	DefaultPlanDir        = ".agents/plans"
	DefaultKnowledgeDir   = ".agents/knowledge"
	DefaultSkillsDir      = ".agents/skills"
	DefaultMode           = "agent"
	DefaultRAGTopK        = 5
	DefaultRAGModel       = "all-minilm"
	DefaultLlamaServerURL = "http://localhost:8090"
	MaxConfigSize         = 10 * 1024 // 10 KB
)

// envVarMap maps environment variables to config field names.
var envVarMap = map[string]string{
	"LOCAL_CLI_MODEL":        "model",
	"LOCAL_CLI_SIDECAR_MODEL": "sidecar_model",
	"LOCAL_CLI_DEBUG":         "debug",
	"LOCAL_CLI_PROVIDER":      "provider",
	"OLLAMA_HOST":             "ollama_host",
	"LOCAL_CLI_NUM_CTX":       "num_ctx",
	"LOCAL_CLI_TEMPERATURE":   "temperature",
	"LOCAL_CLI_TOP_P":         "top_p",
	"LOCAL_CLI_TOP_K":         "top_k",
	"LOCAL_CLI_THINK_MODE":    "think_mode",
	"LOCAL_CLI_KEEP_ALIVE":    "keep_alive",
	"LLAMA_SERVER_URL":        "llama_server_url",
}

// Config holds all resolved configuration values.
type Config struct {
	Model          string
	SidecarModel   string
	OllamaHost     string
	StateDir       string
	ConfigFilePath string
	AutoApprove    bool
	Debug          bool
	RAG            bool
	RAGPath        string
	RAGTopK        int
	RAGModel       string
	Provider       string
	RegistryFile   string
	Orchestrator   string
	PlanDir        string
	KnowledgeDir   string
	SkillsDir      string
	DefaultMode    string
	NumCtx         int
	Temperature    *float64
	TopP           *float64
	TopK           *int
	ThinkMode      bool
	KeepAlive      string
	LlamaServerURL string
}

// CLIArgs holds parsed command-line arguments. Nil pointer means "not set".
type CLIArgs struct {
	Model          *string
	SidecarModel   *string
	OllamaHost     *string
	Debug          *bool
	RAG            *bool
	RAGPath        *string
	RAGTopK        *int
	RAGModel       *string
	Provider       *string
	RegistryFile   *string
	Orchestrator   *string
	NumCtx         *int
	Temperature    *float64
	TopP           *float64
	TopK           *int
	ThinkMode      *bool
	KeepAlive      *string
	LlamaServerURL *string
	DefaultMode    *string
}

// New creates a Config with layered resolution: CLI > env > file > defaults.
func New(cli *CLIArgs) *Config {
	cfg := defaults()

	// Layer 1: config file
	cfgPath := expandHome(cfg.ConfigFilePath)
	if fileVals, err := loadConfigFile(cfgPath); err == nil {
		applyFileValues(cfg, fileVals)
	}

	// Layer 2: environment variables
	applyEnvValues(cfg)

	// Layer 3: CLI arguments (highest priority)
	if cli != nil {
		applyCLIArgs(cfg, cli)
	}

	// Validate model name
	if cfg.Model != "" && !security.ValidateModelName(cfg.Model) {
		cfg.Model = DefaultModel
	}

	// Expand paths
	cfg.StateDir = expandHome(cfg.StateDir)
	cfg.ConfigFilePath = expandHome(cfg.ConfigFilePath)

	return cfg
}

// HasClaudeAccess checks if ANTHROPIC_API_KEY is set.
func (c *Config) HasClaudeAccess() bool {
	return os.Getenv("ANTHROPIC_API_KEY") != ""
}

func defaults() *Config {
	return &Config{
		Model:          DefaultModel,
		OllamaHost:     DefaultOllamaHost,
		StateDir:       DefaultStateDir,
		ConfigFilePath: DefaultConfigFile,
		Provider:       DefaultProvider,
		NumCtx:         DefaultNumCtx,
		RAGTopK:        DefaultRAGTopK,
		RAGModel:       DefaultRAGModel,
		RAGPath:        ".",
		PlanDir:        DefaultPlanDir,
		KnowledgeDir:   DefaultKnowledgeDir,
		SkillsDir:      DefaultSkillsDir,
		DefaultMode:    DefaultMode,
		LlamaServerURL: DefaultLlamaServerURL,
	}
}

func loadConfigFile(path string) (map[string]string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	// Reject symlinks
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("config file is a symlink: %s", path)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("config file is not regular: %s", path)
	}
	if info.Size() > MaxConfigSize {
		return nil, fmt.Errorf("config file exceeds %d bytes: %s", MaxConfigSize, path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vals := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		vals[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return vals, scanner.Err()
}

func applyFileValues(cfg *Config, vals map[string]string) {
	if v, ok := vals["model"]; ok {
		cfg.Model = v
	}
	if v, ok := vals["sidecar_model"]; ok {
		cfg.SidecarModel = v
	}
	if v, ok := vals["ollama_host"]; ok {
		cfg.OllamaHost = v
	}
	if v, ok := vals["state_dir"]; ok {
		cfg.StateDir = v
	}
	if v, ok := vals["provider"]; ok {
		cfg.Provider = v
	}
	if v, ok := vals["debug"]; ok {
		cfg.Debug = parseBool(v)
	}
	if v, ok := vals["auto_approve"]; ok {
		cfg.AutoApprove = parseBool(v)
	}
	if v, ok := vals["num_ctx"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.NumCtx = n
		}
	}
	if v, ok := vals["temperature"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Temperature = &f
		}
	}
	if v, ok := vals["top_p"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.TopP = &f
		}
	}
	if v, ok := vals["top_k"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.TopK = &n
		}
	}
	if v, ok := vals["think_mode"]; ok {
		cfg.ThinkMode = parseBool(v)
	}
	if v, ok := vals["keep_alive"]; ok {
		cfg.KeepAlive = v
	}
	if v, ok := vals["llama_server_url"]; ok {
		cfg.LlamaServerURL = v
	}
	if v, ok := vals["rag"]; ok {
		cfg.RAG = parseBool(v)
	}
	if v, ok := vals["rag_path"]; ok {
		cfg.RAGPath = v
	}
	if v, ok := vals["rag_topk"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RAGTopK = n
		}
	}
	if v, ok := vals["rag_model"]; ok {
		cfg.RAGModel = v
	}
	if v, ok := vals["default_mode"]; ok {
		cfg.DefaultMode = v
	}
}

func applyEnvValues(cfg *Config) {
	for envKey, cfgKey := range envVarMap {
		val := os.Getenv(envKey)
		if val == "" {
			continue
		}
		switch cfgKey {
		case "model":
			cfg.Model = val
		case "sidecar_model":
			cfg.SidecarModel = val
		case "debug":
			cfg.Debug = parseBool(val)
		case "provider":
			cfg.Provider = val
		case "ollama_host":
			cfg.OllamaHost = val
		case "num_ctx":
			if n, err := strconv.Atoi(val); err == nil {
				cfg.NumCtx = n
			}
		case "temperature":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				cfg.Temperature = &f
			}
		case "top_p":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				cfg.TopP = &f
			}
		case "top_k":
			if n, err := strconv.Atoi(val); err == nil {
				cfg.TopK = &n
			}
		case "think_mode":
			cfg.ThinkMode = parseBool(val)
		case "keep_alive":
			cfg.KeepAlive = val
		case "llama_server_url":
			cfg.LlamaServerURL = val
		}
	}
}

func applyCLIArgs(cfg *Config, cli *CLIArgs) {
	if cli.Model != nil {
		cfg.Model = *cli.Model
	}
	if cli.SidecarModel != nil {
		cfg.SidecarModel = *cli.SidecarModel
	}
	if cli.OllamaHost != nil {
		cfg.OllamaHost = *cli.OllamaHost
	}
	if cli.Debug != nil {
		cfg.Debug = *cli.Debug
	}
	if cli.RAG != nil {
		cfg.RAG = *cli.RAG
	}
	if cli.RAGPath != nil {
		cfg.RAGPath = *cli.RAGPath
	}
	if cli.RAGTopK != nil {
		cfg.RAGTopK = *cli.RAGTopK
	}
	if cli.RAGModel != nil {
		cfg.RAGModel = *cli.RAGModel
	}
	if cli.Provider != nil {
		cfg.Provider = *cli.Provider
	}
	if cli.RegistryFile != nil {
		cfg.RegistryFile = *cli.RegistryFile
	}
	if cli.Orchestrator != nil {
		cfg.Orchestrator = *cli.Orchestrator
	}
	if cli.NumCtx != nil {
		cfg.NumCtx = *cli.NumCtx
	}
	if cli.Temperature != nil {
		cfg.Temperature = cli.Temperature
	}
	if cli.TopP != nil {
		cfg.TopP = cli.TopP
	}
	if cli.TopK != nil {
		cfg.TopK = cli.TopK
	}
	if cli.ThinkMode != nil {
		cfg.ThinkMode = *cli.ThinkMode
	}
	if cli.KeepAlive != nil {
		cfg.KeepAlive = *cli.KeepAlive
	}
	if cli.LlamaServerURL != nil {
		cfg.LlamaServerURL = *cli.LlamaServerURL
	}
	if cli.DefaultMode != nil {
		cfg.DefaultMode = *cli.DefaultMode
	}
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "1" || s == "true" || s == "yes"
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
