// Package cli implements the interactive REPL and slash commands.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"log/slog"

	"go.muehmer.eu/lai/internal/agent"
	"go.muehmer.eu/lai/internal/config"
	"go.muehmer.eu/lai/internal/health"
	"go.muehmer.eu/lai/internal/provider"
	"go.muehmer.eu/lai/internal/provider/ollama"
	"go.muehmer.eu/lai/internal/security"
	"go.muehmer.eu/lai/internal/token"
	"go.muehmer.eu/lai/internal/tool"
)

const version = "0.1.0-go"

// slashCommands maps command names to help descriptions.
var slashCommands = map[string]string{
	"/help":    "Show this help message.",
	"/exit":    "Exit the REPL.",
	"/quit":    "Exit the REPL (alias for /exit).",
	"/clear":   "Clear conversation history.",
	"/model":   "Switch to a different model. Usage: /model <name>",
	"/status":  "Show current model, message count, connection status.",
	"/models":  "List available models.",
	"/context": "Show context window usage (messages, tokens).",
	"/usage":   "Show per-message token usage and session totals.",
}

// replContext holds mutable state for the REPL loop.
type replContext struct {
	config  *config.Config
	prov    provider.Provider
	client  *ollama.Client
	tools   []tool.Tool
	msgs    []provider.Message
	tracker *token.Tracker
	limiter *agent.RateLimiter
}

// BuildSystemPrompt generates the system prompt with tool descriptions.
func BuildSystemPrompt(tools []tool.Tool) string {
	var sb strings.Builder
	sb.WriteString("You are a helpful AI coding agent. You have access to the following tools:\n\n")
	for _, t := range tools {
		fmt.Fprintf(&sb, "- %s: %s\n", t.Name(), t.Description())
	}
	sb.WriteString("\n## How to work\n")
	sb.WriteString("1. Understand the user's request\n")
	sb.WriteString("2. Use tools to gather information (read files, search code)\n")
	sb.WriteString("3. Make changes using tools (write, edit, bash)\n")
	sb.WriteString("4. Verify your changes work\n")
	sb.WriteString("\n## Rules\n")
	sb.WriteString("- ALWAYS use tools to interact with the filesystem. Never guess file contents.\n")
	sb.WriteString("- Read files before editing them.\n")
	sb.WriteString("- Use grep/glob to find files before reading them.\n")
	sb.WriteString("- After making changes, verify they work (run tests, build, etc.).\n")
	sb.WriteString("- Be concise in your responses.\n")

	cwd, _ := os.Getwd()
	fmt.Fprintf(&sb, "\nWorking directory: %s\n", cwd)

	return sb.String()
}

// Run starts the interactive REPL.
func Run(cfg *config.Config, prov provider.Provider, client *ollama.Client, tools []tool.Tool) {
	ctx := &replContext{
		config:  cfg,
		prov:    prov,
		client:  client,
		tools:   tools,
		tracker: token.NewTracker(),
		limiter: agent.NewRateLimiter(
			cfg.MaxToolCallsPerTurn,
			cfg.MaxToolCallsTotal,
			cfg.MaxBashPerTurn,
		),
	}

	// Print welcome banner
	printBanner(cfg, tools)

	// Health check
	if client != nil {
		results := health.RunAll(client, cfg.Model)
		fmt.Fprint(os.Stderr, health.Format(results, true))
		fmt.Fprintln(os.Stderr)
	}

	// Initialize conversation with system prompt
	systemPrompt := BuildSystemPrompt(tools)
	ctx.msgs = []provider.Message{
		{Role: "system", Content: systemPrompt},
	}

	slog.Info("audit",
		slog.Group("event",
			slog.String("type", "session_start"),
			slog.String("model", cfg.Model),
			slog.String("provider", cfg.Provider),
		),
	)

	// REPL loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprint(os.Stderr, "You> ")
		if !scanner.Scan() {
			break // EOF
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle slash commands
		if strings.HasPrefix(input, "/") {
			if !handleSlashCommand(input, ctx) {
				return // /exit or /quit
			}
			continue
		}

		// Build user message
		ctx.msgs = append(ctx.msgs, provider.Message{
			Role:    "user",
			Content: input,
		})

		// Build chat options from config
		opts := &provider.ChatOptions{
			NumCtx: cfg.NumCtx,
		}
		if cfg.Temperature != nil {
			opts.Temperature = cfg.Temperature
		}
		if cfg.TopP != nil {
			opts.TopP = cfg.TopP
		}
		if cfg.TopK != nil {
			opts.TopK = cfg.TopK
		}
		if cfg.ThinkMode {
			think := true
			opts.Think = &think
		}

		// Run agent loop
		if err := agent.Loop(prov, cfg.Model, tools, &ctx.msgs, ctx.tracker, opts, ctx.limiter); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}

func handleSlashCommand(input string, ctx *replContext) bool {
	cmd, arg, _ := strings.Cut(input, " ")
	arg = strings.TrimSpace(arg)

	switch cmd {
	case "/exit", "/quit":
		fmt.Fprintln(os.Stderr, "Goodbye!")
		return false

	case "/help":
		fmt.Fprintln(os.Stderr, "Available commands:")
		for cmd, desc := range slashCommands {
			fmt.Fprintf(os.Stderr, "  %-12s %s\n", cmd, desc)
		}

	case "/clear":
		systemPrompt := BuildSystemPrompt(ctx.tools)
		ctx.msgs = []provider.Message{
			{Role: "system", Content: systemPrompt},
		}
		ctx.tracker.Clear()
		ctx.limiter.ResetAll()
		fmt.Fprintln(os.Stderr, "Conversation cleared.")

	case "/model":
		if arg == "" {
			fmt.Fprintf(os.Stderr, "Current model: %s\n", ctx.config.Model)
		} else if !security.ValidateModelName(arg) {
			fmt.Fprintln(os.Stderr, "Error: invalid model name format.")
		} else {
			old := ctx.config.Model
			ctx.config.Model = arg
			fmt.Fprintf(os.Stderr, "Switched to model: %s\n", arg)
			slog.Info("audit",
				slog.Group("event",
					slog.String("type", "model_switch"),
					slog.String("from", old),
					slog.String("to", arg),
				),
			)
		}

	case "/status":
		fmt.Fprintf(os.Stderr, "Model: %s\n", ctx.config.Model)
		fmt.Fprintf(os.Stderr, "Provider: %s\n", ctx.prov.Name())
		fmt.Fprintf(os.Stderr, "Messages: %d\n", len(ctx.msgs))
		fmt.Fprintf(os.Stderr, "Tokens: ~%d\n", agent.EstimateTokens(ctx.msgs))
		turn, total := ctx.limiter.Stats()
		fmt.Fprintf(os.Stderr, "Tool calls: %d this turn, %d total (limits: %d/%d)\n",
			turn, total, ctx.config.MaxToolCallsPerTurn, ctx.config.MaxToolCallsTotal)

	case "/models":
		models, err := ctx.prov.ListModels()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		} else {
			fmt.Fprintln(os.Stderr, "Available models:")
			for _, m := range models {
				fmt.Fprintf(os.Stderr, "  %s\n", m.Name)
			}
		}

	case "/context":
		fmt.Fprintf(os.Stderr, "Messages: %d\n", len(ctx.msgs))
		fmt.Fprintf(os.Stderr, "Estimated tokens: %d\n", agent.EstimateTokens(ctx.msgs))
		fmt.Fprintf(os.Stderr, "Context window: %d\n", ctx.config.NumCtx)

	case "/usage":
		fmt.Fprintln(os.Stderr, ctx.tracker.FormatTable())
		fmt.Fprintln(os.Stderr, ctx.tracker.FormatSummary())

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s. Type /help for available commands.\n", cmd)
	}

	return true
}

func printBanner(cfg *config.Config, tools []tool.Tool) {
	fmt.Fprintf(os.Stderr, "\n  lai v%s — Local AI Agent (Go)\n", version)
	fmt.Fprintf(os.Stderr, "  Model: %s | Provider: %s\n", cfg.Model, cfg.Provider)

	toolNames := make([]string, len(tools))
	for i, t := range tools {
		toolNames[i] = t.Name()
	}
	fmt.Fprintf(os.Stderr, "  Tools: %s\n", strings.Join(toolNames, ", "))
	fmt.Fprintf(os.Stderr, "  Type /help for commands, /exit to quit.\n\n")
}
