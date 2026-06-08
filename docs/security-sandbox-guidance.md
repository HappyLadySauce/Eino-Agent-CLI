# Security Sandbox Guidance

This document is the implementation guide for Eino-Agent-CLI's future security sandbox, approval layer, command tools, file tools, rule engine, MCP integration, memory system, session persistence, and self-iteration features.

The security layer must be the mandatory entry point for every tool call. Individual tools must not decide their own safety policy or bypass shared enforcement.

## Background And Goals

Eino-Agent-CLI is moving toward a larger tool ecosystem. File operations, shell commands, MCP tools, memory writes, session persistence, and self-iteration can all modify local or external state. A prompt-only safety model is not enough for these capabilities.

The target model is a two-layer design:

- Sandbox answers: can this operation access this resource at all?
- Approval answers: if the sandbox allows it, should the user or policy approve this operation before execution?

This keeps hard safety boundaries separate from user interaction and automation policy. It also makes future tools easier to add because every tool goes through the same policy chain.

## Core Principles

- Keep `SessionMode` and `SandboxMode` separate.
- `SessionMode` describes user intent and the main agent's responsibility.
- `SandboxMode` describes the system capability ceiling.
- Sandbox evaluation runs before approval evaluation.
- Deny decisions always take precedence over ask and allow decisions.
- All tool calls must pass through one policy path.
- Tool failures, denials, and approval rejections must return structured results so the model can correct its next action.
- `~/eino` is the data directory for future sessions, memory, audit logs, and rules. It is not the default workspace root.
- The workspace root is the CLI startup directory and remains fixed for the session. The model cannot change it.

## Permission Model

### SessionMode

`ask`

- Allows file read/list tools.
- Rejects command execution.
- Rejects file write/delete.
- Rejects create/run subagent operations.
- Intended for questions, explanations, and code reading.

`plan`

- Allows file read/list tools.
- Allows conservative read-only commands.
- Rejects or returns suggestions for file write/delete and non-read-only commands.
- Intended for research and implementation planning without changing state.

`agent`

- Allows work inside the workspace boundary.
- Allows file read/list/create.
- Requires approval or rule allow for file update/delete.
- Requires approval or rule allow for non-read-only commands.
- Intended for implementation work with controlled side effects.

### SandboxMode

`read-only`

- Allows read-only file operations inside the workspace.
- Rejects file write/delete.
- Rejects command execution by default.
- Suitable for `/ask` and conservative `/plan` sessions.

`workspace-write`

- Allows reads and writes inside the workspace.
- Marks delete, overwrite, command execution, and external access as policy-sensitive.
- Suitable for normal `/agent` implementation sessions.

`danger-full-access`

- Removes normal workspace restrictions only when explicitly enabled by the user.
- Still keeps hard-deny protections unless the user explicitly disables them in configuration.
- Intended only for isolated containers, VMs, or trusted automation environments.

### ApprovalMode

`interactive`

- Ask the user before policy-sensitive operations.
- CLI prompts must show the tool name, arguments summary, cwd or target path, risk, and reason.

`auto`

- Automatically approves trusted operations that stay within sandbox boundaries.
- Risky or unknown operations still ask or deny according to policy.

`auto-edit`

- Automatically approves workspace-local create/update operations.
- File delete and non-read-only commands still ask unless rules explicitly allow them.

`suggest`

- Does not execute write/delete/command operations.
- Returns a structured suggestion describing what would have happened.
- Useful for `/plan`.

`never`

- Never prompts the user.
- Any operation that would require approval is denied.
- Useful for CI, headless runs, and scripts.

## Tool Security Specification

### ToolDescriptor

Every built-in, plugin, and MCP tool must register a descriptor before execution:

```go
type ToolDescriptor struct {
	Name        string
	Provider    string // builtin, mcp, plugin
	Kind        ToolKind
	Risk        OperationRisk
	AutoApprove bool
}
```

`AutoApprove` is a tool-level default, not an escape hatch. Sandbox hard-deny and explicit deny rules still win.

### File Tools

Prefer explicit tools over one broad write API:

- `read_file(path)`
- `list_dir(path)`
- `create_file(path, content)`
- `patch_file(path, patch)`
- `replace_file(path, content)`
- `delete_file(path)`

Rules:

- `read_file` and `list_dir` are allowed in `/ask`, `/plan`, and `/agent` when the path is inside the workspace and not denied by rules.
- `create_file` is allowed in `/agent` under `workspace-write`; it may be auto-approved depending on `ApprovalMode`.
- `patch_file` is preferred over `replace_file` because it is easier to inspect and approve.
- `replace_file` is higher risk than `patch_file` because it overwrites full contents.
- `delete_file` is destructive and must require approval or an explicit rule allow.
- File tools must normalize paths with `filepath.Clean`, `filepath.Abs`, and symlink evaluation before boundary checks.
- Path containment must reject `..` escapes, absolute path escapes, and symlink escapes.

### Command Tool

`run_command(command, cwd, timeout_seconds)` must always pass through command classification, sandbox checks, approval policy, and audit logging.

Rules:

- Empty `cwd` defaults to workspace root.
- `cwd` must be inside the workspace unless `danger-full-access` is enabled.
- Windows execution uses PowerShell.
- Linux and macOS execution uses `/bin/sh -c`.
- Commands must have a timeout.
- Output must have a maximum byte limit and return `truncated=true` when exceeded.

Only simple commands can be auto-allowed by prefix rules. A command is not simple if it contains chaining, redirection, pipes, command substitution, or shell-specific control operators.

Escalate to ask or deny when a command contains:

- `;`
- `&&`
- `||`
- `|`
- `>`
- `>>`
- `$()`
- backticks
- PowerShell pipeline or statement chaining
- `Invoke-*`
- `Remove-*`
- `Set-*`
- network commands
- install commands
- deploy or push commands
- unknown commands

Examples:

- `git status` can be read-only.
- `git diff` can be read-only.
- `git fetch` can be read-only if explicitly allowed.
- `git push` is state-changing and must not be auto-approved.
- `go test ./...` can be allowed as a read-like verification command, but commands that write generated files should be classified more strictly.
- `git status; rm -rf .` must never match a `git status` allow rule.

### Agent Tools

Existing agent orchestration tools are low risk by default:

- `list_agents`
- `create_agent`
- `run_subagent`

Current policy:

- `/ask` rejects create/run subagent.
- `/plan` allows only readonly/plan permission subagents.
- `/agent` allows subagents according to their declared permission.

These tools still need descriptors and audit entries so future changes do not create a parallel tool path.

### MCP And Plugin Tools

MCP and plugin tools must use the same descriptor, sandbox, approval, and audit pipeline as built-in tools.

Do not add a separate "trusted external tool" path. External tools can read databases, call APIs, write remote state, or exfiltrate data. They must declare:

- provider
- tool kind
- risk
- external domains or resources
- whether they support dry-run or suggest mode
- whether they expose destructive operations

Future policy should support domain allowlists and per-provider rules.

## Risk Handling

### Structured Tool Results

Every tool should return structured status instead of plain text only:

```json
{
  "ok": false,
  "status": "denied",
  "reason": "command requires approval in plan mode",
  "suggestion": "Switch to agent mode or ask the user to approve the operation."
}
```

Recommended statuses:

- `ok`
- `denied`
- `approval_required`
- `rejected`
- `failed`
- `timeout`
- `truncated`
- `suggested`

This lets the model revise behavior instead of retrying the same blocked action.

### Hard Deny

Hard-deny protections apply even in `danger-full-access` unless the user explicitly disables them.

Default hard-deny targets:

- deleting the workspace root
- deleting the home directory
- deleting a filesystem root or drive root
- modifying critical `.git` internals
- reading private keys such as `.ssh/id_*`
- reading known secret files such as `.env` unless explicitly allowed
- uploading sensitive files to a network target
- running commands that recursively delete broad paths

### Dry-Run And Suggest Mode

Tools with side effects should support dry-run or suggest mode where practical:

- `delete_file(dry_run=true)` returns what would be deleted.
- `replace_file(dry_run=true)` returns whether the file exists and whether content would change.
- `run_command(dry_run=true)` returns risk classification and approval requirements without executing.

`/plan` should prefer suggest behavior for operations that would mutate state.

### Approval Prompt Fields

Approval prompts must provide enough context for a user to make a decision:

```text
Tool: run_command
CWD: D:\Code\project
Input: {"command":"rm -rf ./build"}
Risk: destructive
Reason: Destructive command removes files under ./build.
Decision: approve once? [y/N]
```

Prompt text should be concise, but not hide risk reasons.

### Audit Log

The security layer should expose an append-only audit log interface. The first implementation may be in-memory or file-backed later under `~/eino`.

Fields:

- timestamp
- session id
- session mode
- sandbox mode
- approval mode
- tool name
- provider
- cwd
- target path
- arguments summary
- risk
- decision
- reason
- approved or rejected by user
- duration
- result status

Audit logs will support future session persistence, memory review, self-iteration, and debugging.

## Rules Engine Roadmap

The first implementation can use built-in policy. The next stage should add declarative rules inspired by Codex-style exec policy and Claude-style allow/ask/deny rules.

Candidate locations:

- `.eino/rules.star` for project policy
- `~/eino/rules.star` for user policy

Rule examples:

```python
prefix_rule(pattern = ["git", "status"], decision = "allow")
prefix_rule(pattern = ["git", "push"], decision = "ask")
glob_rule(pattern = "*.env", decision = "deny")
tool_rule(tool = "delete_file", decision = "ask")
when(session_mode = "plan", tool = "replace_file", decision = "deny")
```

Evaluation order:

1. hard deny
2. sandbox deny
3. explicit rule deny
4. explicit rule ask
5. explicit rule allow
6. built-in default policy

Rules should support hot reload. If reload fails, keep the last valid rule set and report the parse error.

## Implementation Roadmap

### P0: Sandbox Boundary And Decision Model

- Add `internal/security`.
- Define sandbox modes, tool kinds, operation risks, decisions, and operation metadata.
- Implement workspace root normalization and containment checks.
- Reject path escapes and symlink escapes.
- Add unit tests for path boundaries and mode matrix.

### P1: Approval And Built-In Tools

- Add `internal/approval`.
- Implement CLI prompter and fake prompter tests.
- Add file tools: read, list, create, patch, replace, delete.
- Add command tool with conservative classifier.
- Route all built-in tools through sandbox and approval.
- Add structured tool results.
- Add audit log interface.

### P2: Declarative Rule Engine

- Add rules file loading.
- Implement prefix, glob, tool, and session-mode rules.
- Enforce deny-first evaluation.
- Add sample rules.
- Add tests for precedence and reload failures.

### P3: MCP And Plugin Tool Ecosystem

- Require every MCP/plugin tool to register `ToolDescriptor`.
- Apply the same sandbox, approval, rule, and audit path.
- Add domain/resource policy metadata for external tools.

### P4: Memory, Session Save, And Self-Iteration

- Store session data under `~/eino`.
- Route memory writes through the same policy path.
- Save audit logs alongside sessions.
- Let self-iteration use structured tool results and audit history to revise plans safely.

Do not couple memory/session/self-iteration directly into P0 or P1. They should consume the security framework after it is stable.

## Review Checklist

- `/ask` allows file read/list and rejects command/write/delete/subagent create/run.
- `/plan` allows file read/list and conservative read-only commands.
- `/agent` allows workspace work but gates write/delete/non-read-only commands.
- `~/eino` is documented as the data directory, not workspace root.
- Workspace root is fixed at CLI startup.
- `danger-full-access` retains hard-deny protections by default.
- Prefix allow does not apply to chained, redirected, piped, or substituted commands.
- Tool denials and approvals return structured results.
- MCP/plugin tools must use the same security path as built-in tools.
