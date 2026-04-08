// Command lai is a local-first AI coding agent powered by Ollama.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"go.muehmer.eu/lai/internal/pkg/cli"
	"go.muehmer.eu/lai/internal/pkg/config"
	"go.muehmer.eu/lai/internal/pkg/logging"
	"go.muehmer.eu/lai/internal/pkg/provider/ollama"
	"go.muehmer.eu/lai/internal/pkg/tool"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cliArgs := parseFlags()
	cfg := config.New(cliArgs)

	if cfg.Debug && cfg.LogLevel == config.DefaultLogLevel {
		cfg.LogLevel = "debug"
	}
	if err := logging.Setup(cfg.LogLevel, cfg.LogFile); err != nil {
		return fmt.Errorf("logging setup: %w", err)
	}

	slog.Debug("config loaded",
		"model", cfg.Model,
		"provider", cfg.Provider,
		"ollama_host", cfg.OllamaHost,
		"num_ctx", cfg.NumCtx,
	)

	prov, err := ollama.NewProvider(cfg.OllamaHost)
	if err != nil {
		return fmt.Errorf("provider init: %w", err)
	}

	tools := tool.DefaultTools()
	cli.Run(ctx, cfg, prov, prov.OllamaClient(), tools)
	return nil
}

func parseFlags() *config.CLIArgs {
	args := &config.CLIArgs{}

	model := flag.String("model", "", "Model name (e.g. qwen3:8b)")
	ollamaHost := flag.String("ollama-host", "", "Ollama server URL")
	provider := flag.String("provider", "", "LLM provider (ollama, claude)")
	debug := flag.Bool("debug", false, "Enable debug output")
	numCtx := flag.Int("num-ctx", 0, "Context window size")
	temp := flag.Float64("temperature", -1, "Sampling temperature")
	topP := flag.Float64("top-p", -1, "Top-p sampling")
	topK := flag.Int("top-k", -1, "Top-k sampling")
	thinkMode := flag.Bool("think", false, "Enable extended thinking")
	logLevel := flag.String("log-level", "", "Log level (debug, info, error)")
	logFile := flag.String("log-file", "", "Path to JSON log file")

	flag.Parse()

	if *model != "" {
		args.Model = model
	}
	if *ollamaHost != "" {
		args.OllamaHost = ollamaHost
	}
	if *provider != "" {
		args.Provider = provider
	}
	if flag.Lookup("debug").Value.String() == "true" {
		args.Debug = debug
	}
	if *numCtx > 0 {
		args.NumCtx = numCtx
	}
	if *temp >= 0 {
		args.Temperature = temp
	}
	if *topP >= 0 {
		args.TopP = topP
	}
	if *topK >= 0 {
		args.TopK = topK
	}
	if *thinkMode {
		args.ThinkMode = thinkMode
	}
	if *logLevel != "" {
		args.LogLevel = logLevel
	}
	if *logFile != "" {
		args.LogFile = logFile
	}

	return args
}
