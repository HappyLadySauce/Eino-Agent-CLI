# CLAUDE.md 回复用户说中文

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build, Test, and Run

```bash
# Build
go build ./...

# Run (requires settings.json or env vars for model config)
EINO_AUTH_TOKEN="sk-..." EINO_BASE_URL="https://api.openai.com/v1" EINO_MODEL="gpt-4o" go run ./cmd/eino.go

# All tests
go test ./...

# Single package tests
go test ./internal/agents/...
go test ./internal/commands/...

# Verbose single test
go test -v -run TestParseAgentCommand ./internal/commands/...
```

Go 1.26.1. Module path: `github.com/HappyLadySauce/Eino-Agent-CLI`.

## Configuration

Config is loaded by Viper cascade: `--config` flag > `settings.{json,yaml,yml,toml}` in cwd > same files in `~/eino/` > env vars prefixed with `EINO_`. No merging across locations. Env vars inside config files (`${VAR}`) are expanded.

The [settings.json](settings.json) at the repo root is a template using `${EINO_AUTH_TOKEN}` / `${EINO_BASE_URL}` / `${EINO_MODEL}` expansion.

## Architecture

This is an interactive multi-agent CLI powered by [CloudWeGo Eino ADK](https://github.com/cloudwego/eino). The main agent delegates to dynamically-created sub-agents that run in isolated contexts.

### Entry flow

[cmd/eino.go](cmd/eino.go) → [cmd/app/app.go](cmd/app/app.go) `NewAPICommand()` → `run()` initializes global `config.Config` → `agents.RunAgentLoop()` starts the read-eval-print loop.

### Three session modes

Each mode gets its **own ADK agent + runner**, not a single agent with conditional behavior. Defined in [internal/commands/command.go](internal/commands/command.go):

| Mode | CLI command | Has orchestrator tools? | Sub-agent permissions allowed |
|------|------------|------------------------|-------------------------------|
| `agent` (default) | `/agent` | Yes — `list_agents`, `create_agent`, `run_subagent` | `default`, `readonly`, `plan` |
| `plan` | `/plan` | Yes | `readonly`, `plan` only |
| `ask` | `/ask` | No | None (no sub-agents at all) |

### Core components

- **`AgentRuntime`** ([internal/agents/runtime.go](internal/agents/runtime.go)): Owns the shared OpenAI chat model, the agent registry, and three main-agent runners (one per mode). Also owns sub-agent runners keyed by name. Thread-safe via `sync.RWMutex`.

- **`AgentRegistry`** ([internal/agents/registry.go](internal/agents/registry.go)): In-memory map of `AgentDefinition` by name. Validates names (`^[a-z0-9][a-z0-9-]*$`) and uniqueness. Supports dynamic registration at runtime.

- **`AgentDefinition`** ([internal/agents/definition.go](internal/agents/definition.go)): Name, description, system prompt, `PermissionMode` (`default`/`readonly`/`plan`), and a `Dynamic` flag distinguishing built-in from runtime-created agents.

- **`ContextMiddleware`** ([internal/middlewares/context.go](internal/middlewares/context.go)): ADK middleware that runs `BeforeModelRewriteState` to trim messages to the token budget, `AfterModelRewriteState` to record usage stats, and `WrapModel` to inject a timing wrapper. Safety margin is 5% of budget, clamped to [128, 2048].

### Interactive loop

[internal/agents/interactive.go](internal/agents/interactive.go) — `RunAgentLoop()`:

1. Creates `AgentRuntime` + `ModeState`
2. Reads stdin on a background goroutine (context-cancellation-aware)
3. Parses input via `commands.ParseAgentCommand()` — recognizes `exit`/`quit`, `/agent`/`/plan`/`/ask`, and plain chat
4. Dispatches: mode switches update state; chat invokes `runtime.RunMain()` which streams through an `AnimatedWriter` spinner, then appends the result to `Messages` history
5. After each turn, prints a stats line (elapsed, prompt tokens, completion tokens, turn total, session total, context usage %)

### Sub-agent orchestration

Tools defined in [internal/tools/agent_tools.go](internal/tools/agent_tools.go):
- `list_agents` — list registered agents
- `create_agent` — dynamically register a new sub-agent with permission constraints
- `run_subagent` — run a named sub-agent with **fresh isolated context** (no chat history, just a constructed prompt)

The `AgentRuntime` implements `AgentToolService` — methods like `CreateAgent()`, `RunSubAgent()`, `ListAgents()`. Sub-agents write to `io.Discard`; only the final content is returned to the main agent.

### Token counting

[pkg/utils/tokens/tokens.go](pkg/utils/tokens/tokens.go): Uses `tiktoken-go` for message/tool token estimation. Falls back to `cl100k_base` encoding if the model isn't recognized. Counts 4 overhead tokens per message + role/content/tool calls.

[pkg/utils/messages/messages.go](pkg/utils/messages/messages.go): `TrimByMessageCount` (keep last N), `TrimByTokenBudget` (remove oldest non-latest-user messages until under budget).

### Terminal output

[internal/terminal/terminal.go](internal/terminal/terminal.go): `Style` wraps text in ANSI codes only when the destination is a TTY (respects `NO_COLOR`, `TERM=dumb`, and `ModeCharDevice` check). `AnimatedWriter` shows a spinning `|/-\` animation until the first real output.

[internal/messages/messages.go](internal/messages/messages.go): `channelWriter` routes streaming output through three labeled channels (`Assistant>`, `Assistant[thinking]>`, `Assistant[tools]>`) and splits content on `<|channel>thought ... <channel|>` markers.

### Config file loading

[pkg/options/config.go](pkg/options/config.go): Searches `settings.{json,yaml,yml,toml}` in cwd first, then `~/eino/`. `--config` flag takes precedence; failure with explicit `--config` fails hard (no fallback). `os.ExpandEnv` is applied to config file contents before parsing.

## Security Sandbox (Future)

[docs/security-sandbox-guidance.md](docs/security-sandbox-guidance.md) is a design document for a planned two-layer security system (sandbox + approval). Not yet implemented — future work lives in `internal/security/` and `internal/approval/`. The P0 priority is sandbox boundary enforcement, workspace root containment, and path escape rejection.
