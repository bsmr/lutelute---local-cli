# PLAN: Structured Logging & Rate-Limiting

## Context

The Go prototype has **no logging system** — all output goes directly to stderr/stdout
via `fmt.Fprintf`. Debug output uses `[debug]` prefix behind a boolean flag. There is no
audit trail, no log files, no syslog integration, no structured output.

The Python version is identical: pure `print()` / `sys.stderr.write()`, no `logging` module.

This plan introduces structured logging via Go's `log/slog` (stdlib since 1.21) and
rate-limiting for tool execution in the agent loop.

---

## Phase 1: Structured Logging (`internal/logging`)

### 1.1 Goals

- Replace all `fmt.Fprintf(os.Stderr, "[debug] ...")` with `slog.Debug()`
- Replace all `fmt.Fprintf(os.Stderr, "Error: ...")` with `slog.Error()`
- Keep user-facing UI output (banner, spinner, prompt) on stderr via `fmt` — these are
  **not** log messages
- Structured key-value pairs for machine-parseable output
- Optional file logging alongside terminal output

### 1.2 Log Levels

| Level | Purpose | Examples |
|-------|---------|---------|
| `DEBUG` | Internal state, cache hits, compaction | `[debug] cache hit: read`, `[debug] Compacting 50 messages` |
| `INFO` | Significant operational events | Tool execution, model switch, provider connect |
| `WARN` | Recoverable issues | Model not found locally, disk space low, stream retry |
| `ERROR` | Failures that affect results | Connection failed, tool execution error, invalid input |

### 1.3 Architecture

```
internal/logging/
  logging.go          # Setup function, handler creation
```

**Key decisions:**

- **Handler**: `slog.NewTextHandler` for terminal, `slog.NewJSONHandler` for file output
- **Multi-handler**: When file logging enabled, fan out to both terminal and file
- **Global logger**: Set via `slog.SetDefault()` at startup in `main.go`
- **No new dependency**: `log/slog` is stdlib

**Setup function:**

```go
// Setup configures the global slog logger.
//
//   - level: minimum log level (debug, info, warn, error)
//   - logFile: optional path for JSON log file ("" = no file)
//
// Terminal output uses TextHandler (human-readable).
// File output uses JSONHandler (machine-parseable).
func Setup(level slog.Level, logFile string) error
```

### 1.4 What Gets Logged

**Agent loop** (`internal/agent/`):
```go
slog.Debug("sending messages to model", "count", len(messages), "model", model)
slog.Info("tool called", "tool", funcName, "args_keys", keys(args))
slog.Info("tool result", "tool", funcName, "bytes", len(result), "cached", wasCached)
slog.Debug("compacting messages", "before", len(msgs), "kept_recent", compactKeepRecent)
slog.Error("stream error", "err", err)
```

**Provider** (`internal/provider/ollama/`):
```go
slog.Debug("ollama request", "method", method, "path", path)
slog.Warn("ollama error", "status", resp.StatusCode, "path", path)
```

**Config** (`internal/config/`):
```go
slog.Debug("config loaded", "model", cfg.Model, "provider", cfg.Provider)
slog.Warn("invalid model name in config, using default", "name", name)
```

**Security** (`internal/security/`):
```go
slog.Warn("dangerous command blocked", "command_prefix", cmd[:min(len(cmd), 40)])
slog.Debug("env var stripped", "key", key)  // Key only, never value
```

### 1.5 Audit Trail (Subset of INFO)

For security-relevant events, log at INFO with an `"audit"` group:

```go
slog.Info("audit",
    slog.Group("event",
        slog.String("type", "tool_exec"),
        slog.String("tool", "bash"),
        slog.String("command_prefix", cmd[:min(len(cmd), 80)]),
        slog.Int("exit_code", exitCode),
    ),
)
```

Audit events:
- `tool_exec` — every tool call (tool name, arg summary, result size, duration)
- `model_switch` — model change via /model command
- `provider_connect` — successful connection to Ollama/Claude
- `command_blocked` — dangerous command rejected
- `session_start` / `session_end` — REPL lifecycle

### 1.6 Config Integration

New config keys:

```
log_level=info              # debug, info, warn, error
log_file=                   # empty = no file, path = JSON log file
```

New CLI flags:

```
--log-level string    Log level (debug, info, warn, error). Default: info
--log-file string     Path to JSON log file. Default: none
```

Env vars:

```
LOCAL_CLI_LOG_LEVEL=info
LOCAL_CLI_LOG_FILE=/tmp/lai.log
```

### 1.7 Migration Strategy

1. Create `internal/logging/logging.go` with `Setup()` function
2. Add config keys + CLI flags
3. Call `logging.Setup()` in `main.go` before any other init
4. Replace `fmt.Fprintf(os.Stderr, "[debug] ...")` → `slog.Debug(...)` across all files
5. Replace `fmt.Fprintf(os.Stderr, "Error: ...")` → `slog.Error(...)` where appropriate
6. Keep `fmt.Fprintf(os.Stderr, ...)` for **UI output** (banner, spinner, prompt, tool result display)

**Distinction**: If it's shown to the user as part of the UI → `fmt`. If it's operational/diagnostic → `slog`.

### 1.8 Files to Modify

| File | Changes |
|------|---------|
| `internal/logging/logging.go` | **New** — Setup function, multi-handler |
| `internal/config/config.go` | Add `LogLevel`, `LogFile` fields + parsing |
| `cmd/lai/main.go` | Add flags, call `logging.Setup()` |
| `internal/agent/agent.go` | Replace debug prints → `slog.Debug/Info/Error` |
| `internal/provider/ollama/ollama.go` | Add request/error logging |
| `internal/security/security.go` | Log blocked commands |
| `internal/cli/repl.go` | Log model switches, session events |
| `internal/health/health.go` | Log check results |

---

## Phase 2: Rate-Limiting (`internal/agent/`)

### 2.1 Goals

- Prevent runaway tool execution (LLM stuck in loop or adversarial)
- Configurable limits per session
- Graceful degradation with clear error message to LLM

### 2.2 Design

Add a `RateLimiter` struct to the agent package:

```go
// RateLimiter tracks tool execution counts per agent loop invocation.
type RateLimiter struct {
    maxCallsPerTurn int           // Max tool calls in a single agent loop run
    maxCallsTotal   int           // Max tool calls across entire session
    maxBashPerTurn  int           // Max bash executions per turn (stricter)
    turnCalls       int           // Counter for current turn
    totalCalls      int           // Counter for session
    turnBash        int           // Bash counter for current turn
}
```

**Default limits:**

| Limit | Value | Rationale |
|-------|-------|-----------|
| `maxCallsPerTurn` | 50 | Single user prompt should not trigger 50+ tool calls |
| `maxCallsTotal` | 500 | Entire session cap |
| `maxBashPerTurn` | 20 | Bash is highest-risk tool |

**Integration point** — inside `agent.Loop()`, before each tool execution:

```go
if err := limiter.Check(funcName); err != nil {
    result := fmt.Sprintf("Error: %v. Use /clear to reset limits.", err)
    // Append as tool result and break the loop
}
```

**Reset**: `limiter.ResetTurn()` called at start of each `agent.Loop()` invocation.
`/clear` REPL command resets total counter.

### 2.3 Config Integration

```
max_tool_calls_per_turn=50
max_tool_calls_total=500
max_bash_per_turn=20
```

### 2.4 Files to Modify

| File | Changes |
|------|---------|
| `internal/agent/ratelimit.go` | **New** — RateLimiter struct and methods |
| `internal/agent/agent.go` | Integrate limiter into Loop() |
| `internal/config/config.go` | Add limit fields |
| `internal/cli/repl.go` | Reset total on /clear, show limits in /status |

---

## Phase 3: Syslog (Optional, Future)

If syslog integration is needed later, `log/slog` supports custom handlers.
A syslog handler can be added without changing any call sites:

```go
import "log/syslog"

writer, _ := syslog.New(syslog.LOG_INFO|syslog.LOG_USER, "lai")
// Wrap in slog.Handler that writes to syslog
```

This is **not in scope** for the initial implementation. The JSON log file output
is sufficient for most audit/monitoring needs and can be ingested by any log
aggregator (journald, fluentd, loki, etc.).

---

## Implementation Order

1. **Phase 1.1–1.3**: Create `logging.go`, config keys, CLI flags
2. **Phase 1.4–1.6**: Migrate all debug/error prints to slog
3. **Phase 1.5**: Add audit events
4. **Phase 2**: Rate-limiter
5. **Phase 1.7**: File logging handler (JSON)
6. Tests for all new code

**Estimated scope**: ~300 lines new code, ~100 lines modified across 8 files.

---

## Verification

- `go test ./...` passes
- `./lai --log-level debug` shows structured debug output on stderr
- `./lai --log-file /tmp/lai.log` writes JSON lines to file
- Tool calls beyond limit return clear error to LLM
- `/status` shows current rate-limit counters
- Audit events appear in log file for all tool executions
