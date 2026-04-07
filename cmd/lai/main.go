// Command lai is a local-first AI coding agent powered by Ollama.
package main

import (
	"flag"
	"fmt"
	"os"

	"go.muehmer.eu/lai/internal/cli"
	"go.muehmer.eu/lai/internal/config"
	"go.muehmer.eu/lai/internal/provider/ollama"
	"go.muehmer.eu/lai/internal/tool"
)

func main() {
	cliArgs := parseFlags()
	cfg := config.New(cliArgs)

	if cfg.Debug {
		fmt.Fprintf(os.Stderr, "[debug] Model: %s\n", cfg.Model)
		fmt.Fprintf(os.Stderr, "[debug] Provider: %s\n", cfg.Provider)
		fmt.Fprintf(os.Stderr, "[debug] Ollama host: %s\n", cfg.OllamaHost)
		fmt.Fprintf(os.Stderr, "[debug] NumCtx: %d\n", cfg.NumCtx)
	}

	// Create Ollama provider
	prov, err := ollama.NewProvider(cfg.OllamaHost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Build tool set
	tools := tool.DefaultTools()

	// Run REPL
	cli.Run(cfg, prov, prov.OllamaClient(), tools)
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

	return args
}
