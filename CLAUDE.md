# local-cli (lai) — Project CLAUDE.md

## Overview

Local-first AI coding agent powered by Ollama. Dual implementation:
- **Python** (`local_cli/`): Production version, zero external dependencies
- **Go** (`cmd/lai/`, `internal/`): Prototype port, zero external dependencies

## Style Guides

- **Go**: [Google Go Style Guide](https://google.github.io/styleguide/go/)
- **Python**: [Google Python Style Guide](https://google.github.io/styleguide/pyguide.html)
- **Shell**: [Google Shell Style Guide](https://google.github.io/styleguide/shellguide.html)

## Build & Test

```bash
# Go
go build -o lai ./cmd/lai/       # Build binary
go test ./...                     # Run all tests
go vet ./...                      # Static analysis

# Python
python -m pytest tests/           # Run Python tests
python -m local_cli               # Run Python version
```

## Architecture (Go)

```
cmd/lai/main.go                   # Entry point, flag parsing
internal/
  config/config.go                # Layered config: CLI > env > file > defaults
  provider/
    provider.go                   # Provider interface, message types, error types
    ollama/ollama.go              # Ollama HTTP client + Provider implementation
  tool/
    tool.go                       # Tool interface, FormatTools(), ToolMap()
    tools.go                      # DefaultTools(), SubAgentTools()
    bash.go, read.go, write.go    # Tool implementations
    edit.go, glob.go, grep.go
  agent/agent.go                  # Agent loop: stream → collect → execute tools → repeat
  cli/repl.go                     # Interactive REPL, slash commands, system prompt
  security/security.go            # Command validation, env sanitization, host validation
  spinner/spinner.go              # Braille-dot spinner (goroutine + channel)
  token/tracker.go                # Token usage accounting and cost estimation
  health/health.go                # Startup diagnostics (Ollama, model, disk)
```

### Key Design Decisions

- **Streaming via channels**: Python generators → Go `chan ChatChunk` + goroutine
- **Typed messages**: Python `dict[str, Any]` → Go `provider.Message` struct
- **Error handling**: Python exception hierarchy → Go sentinel errors + `errors.Is()`/`errors.As()`
- **Tool interface**: Python ABC → Go `tool.Tool` interface

## Security Checklist

> Status: `[ ]` = open, `[x]` = fixed, `[~]` = partially addressed

### Critical

- [x] **Edit tool: no path validation** — `edit.go` now calls `isPathSafe()` before read/write.
- [x] **Model name not validated on load** — `config.go` validates via `security.ValidateModelName()`,
      falls back to default. `/model` REPL command also validates.
- [x] **Dangerous command patterns easily bypassed** — `security.go` now normalizes whitespace
      before matching patterns.

### High

- [x] **Read tool: no output size limit** — Added 5 MB cap in `read.go`.
- [x] **Grep tool: no output size limit** — Added 10 MB cumulative byte limit in `grep.go`.
- [x] **Glob tool: unbounded directory recursion** — `walkGlob()` now has `maxWalkDepth=50`.
- [x] **Glob: no `..` rejection in patterns** — Rejects patterns containing `../`.

### Medium

- [x] **Ollama error responses expose full body** — `ollama.go` now truncates error bodies
      to 500 chars.
- [x] **Env sanitization incomplete** — Added `HUGGINGFACE_TOKEN`, `HF_TOKEN`, `DOCKER_CONFIG`,
      `REGISTRY_AUTH_FILE`, `KAGGLE_KEY` + generic pattern matching for `*_SECRET_*`,
      `*_TOKEN`, `*_API_KEY`, `*_PASSWORD`, `*_CREDENTIAL`.
- [x] **Write tool: hardcoded 0644 permissions** — Sensitive filenames (`.env`, `credentials`,
      `id_rsa`, `id_ed25519`, `.netrc`, `.pgpass`, `.my.cnf`) now use 0600.
- [ ] **Agent JSON parse failures silent** — `agent.go` silently ignores malformed tool
      arguments from the LLM. → Planned: will use `slog.Debug()` after logging is implemented.
- [ ] **No per-chunk timeout on streaming** — Only total stream timeout (600s). A stalled
      stream holds resources indefinitely within that window.

### Low

- [ ] **Binary detection: null-byte only** — Misses UTF-16, non-UTF-8 encodings. Consider
      `utf8.Valid()` as secondary check.
- [ ] **No audit logging** — Planned in `PLAN.md` Phase 1.5 (slog-based audit trail).
- [ ] **No rate limiting on tool execution** — Planned in `PLAN.md` Phase 2 (RateLimiter).

## Conventions

- All tool results are strings — errors returned as `"Error: ..."` prefix, never panics.
- Provider errors use sentinel wrapping: `errors.Is(err, provider.ErrConnection)`.
- Config nil pointers mean "not set" (use defaults). Non-nil means explicit override.
- Streaming uses `<-chan ChatChunk` + `<-chan error` (two channels, consumer checks both).
- System prompt is built dynamically from available tools at REPL startup.
