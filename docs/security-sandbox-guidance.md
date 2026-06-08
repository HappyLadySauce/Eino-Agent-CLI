# Security Sandbox Guidance

This document is the implementation guide for Eino-Agent-CLI's future security sandbox, approval layer, command tools, file tools, rule engine, MCP integration, memory system, session persistence, and self-iteration features.

The security layer must be the mandatory entry point for every tool call. Individual tools must not decide their own safety policy or bypass shared enforcement.

## Background And Goals

Eino-Agent-CLI is moving toward a larger tool ecosystem. File operations, shell commands, MCP tools, memory writes, session persistence, and self-iteration can all modify local or external state. A prompt-only safety model is not enough for these capabilities.

The target model is a two-layer design:

- Sandbox answers: can this operation access this resource at all?
- Approval answers: if the sandbox allows it, should the user or policy approve this operation before execution?

This keeps hard safety boundaries separate from user interaction and automation policy. It also makes future tools easier to add because every tool goes through the same policy chain.

## Target Architecture

The security layer should be implemented as a mandatory runtime wrapper around every tool registration. The model, agent runtime, Eino tool adapter, built-in tools, MCP tools, and plugin tools must not call tool handlers directly.

Recommended module layout:

- `internal/security`: core policy types, sandbox checks, hard-deny checks, path containment, command classification, network policy metadata, rate limits, operation metadata, and structured decisions.
- `internal/approval`: interactive and non-interactive approval modes, prompt rendering, fake prompter, and approval decision tests.
- `internal/audit`: append-only audit sink interface, in-memory sink, redaction helpers, and future file-backed sink.
- `internal/rules`: declarative rule loading, validation, hot reload, and rule evaluation.
- `internal/tools`: tool descriptors, tool input/output schemas, built-in tool implementations, and secure tool registration helpers.
- `internal/agents`: agent runtime wiring only; it should receive already secured tools from the secure registry.

The mandatory call path is:

```text
Agent runtime
  -> SecureToolRegistry.Register(descriptor, handler)
  -> SecureToolWrapper.Invoke(ctx, rawInput)
  -> Build OperationRequest
  -> HardDenyPolicy.Evaluate
  -> SandboxPolicy.Evaluate
  -> RulePolicy.Evaluate
  -> ApprovalPolicy.Evaluate
  -> AuditSink.Record pre-decision
  -> Tool handler execution or structured denial/suggestion
  -> AuditSink.Record result
  -> Structured ToolResult returned to model
```

No tool should expose a raw `func(ctx, input) (output, error)` directly to Eino. The Eino adapter should only receive secured wrappers. This is the main enforcement point that prevents future bypasses.

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
- Security configuration must have one source of truth for each session. Tools should receive immutable security context through dependency injection, not through package globals.
- Unknown tools, unknown command shapes, unknown external resources, and failed policy parsing must fail closed by default.

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
- Rejects command execution by default, except explicitly classified local read-only commands when the active session policy allows them.
- Suitable for `/ask` and conservative `/plan` sessions.

`workspace-write`

- Allows reads and writes inside the workspace.
- Marks delete, overwrite, command execution, and external access as policy-sensitive. Policy-sensitive means the sandbox does not hard-deny the operation, but the built-in default policy must return `ask`, `suggested`, or `denied` unless an explicit rule allows it.
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

- Automatically approves operations only when the built-in default policy or an explicit rule says the operation is auto-approvable.
- The exact auto-approved scope is defined by the decision matrix and rules, not by the approval mode itself.
- Risky, unknown, destructive, network, and external operations still ask or deny according to policy.

`never`

- Never prompts the user.
- Any operation that would require approval is denied.
- Useful for CI, headless runs, and scripts.

`suggested` is a tool result status, not an approval mode. `/plan` should prefer `suggested` results for operations that would mutate local or external state.

### SecurityContext

Each session should construct one immutable security context at startup:

```go
type SecurityContext struct {
	SessionID     string
	WorkspaceRoot string
	DataDir       string
	SessionMode   SessionMode
	SandboxMode   SandboxMode
	ApprovalMode  ApprovalMode
}
```

Rules:

- `SessionID` is generated when `AgentRuntime` is created and must be carried through every tool call, approval prompt, structured result, and audit record. Use UUID v4 or ULID.
- `WorkspaceRoot` is the normalized CLI startup directory and must stay fixed for the session.
- `DataDir` defaults to `~/eino` and is never treated as the workspace root.
- Session commands such as `/ask`, `/plan`, and `/agent` may change `SessionMode`, but must not silently raise `SandboxMode` or `ApprovalMode`.
- Raising `SandboxMode` or relaxing `ApprovalMode` must be an explicit user or configuration action.
- Sub-agents inherit the parent sandbox ceiling. A sub-agent can request a lower permission mode, but cannot increase the parent security context.

Recommended default combinations:

| SessionMode | SandboxMode | ApprovalMode | Notes |
| --- | --- | --- | --- |
| `ask` | `read-only` | `never` | Read/list only. No prompts for writes or commands because they are denied. |
| `plan` | `read-only` | `interactive` or `never` | Read/list and explicitly classified local read-only commands. Mutating operations return `suggested` instead of executing. |
| `agent` | `workspace-write` | `interactive` | Normal implementation mode. Sensitive operations ask unless rules allow. |
| `agent` | `workspace-write` | `auto` | Optional automation mode. Exact auto scope comes from the decision matrix and rules. |
| any | `danger-full-access` | `interactive` or `never` | Must be explicitly enabled. Hard-deny remains active by default. |

Invalid or suspicious combinations:

- `/ask` with `workspace-write` or `auto` should be rejected during configuration validation.
- `/plan` with `auto` should be rejected unless the configuration explicitly limits auto approval to read-only operations.
- `danger-full-access` with broad auto approval should require explicit configuration and a visible startup warning.

### Built-In Default Decision Matrix

The built-in default policy is evaluated after hard deny, sandbox deny, and explicit rules. It must provide deterministic behavior for each `(SessionMode, SandboxMode, ToolKind, OperationRisk)` combination.

Decision meanings:

- `allow`: execute without user approval.
- `ask`: request user approval when `ApprovalMode=interactive`; deny when `ApprovalMode=never`.
- `auto-eligible`: execute only when `ApprovalMode=auto` or an explicit allow rule applies; otherwise ask in interactive sessions.
- `suggest`: do not execute; return a structured `suggested` result describing the intended operation.
- `deny`: do not execute.

Baseline matrix:

| SessionMode | SandboxMode | ToolKind | Low risk | Medium risk | High risk | Destructive / external unknown |
| --- | --- | --- | --- | --- | --- | --- |
| `ask` | `read-only` | file read/list | allow | ask for sensitive paths | deny | deny |
| `ask` | any | command, file write/delete, agent create/run, external | deny | deny | deny | deny |
| `plan` | `read-only` | file read/list | allow | ask for sensitive paths | deny | deny |
| `plan` | `read-only` | local read-only command | allow only for known parsed commands | ask | deny | deny |
| `plan` | any | file write/delete, network, external state | suggest | suggest | deny | deny |
| `agent` | `workspace-write` | file create/update inside workspace | auto-eligible | ask | ask | deny |
| `agent` | `workspace-write` | file delete/replace | ask | ask | ask | deny |
| `agent` | `workspace-write` | local command | auto-eligible only for known read-like commands | ask | ask | deny |
| `agent` | `workspace-write` | network/MCP/plugin external | ask with declared resources | ask | ask or deny | deny |
| any | `danger-full-access` | any non-hard-denied operation | ask unless explicitly auto-eligible | ask | ask | ask or deny |

`ApprovalMode` is applied after the matrix:

- `interactive`: `ask` prompts once; first implementation should support only approve-once and deny.
- `auto`: `auto-eligible` executes; `ask` still prompts unless an explicit rule allows it.
- `never`: `ask` and `auto-eligible` both become `denied` unless the matrix says `allow`.

Explicit rules can lower or raise a decision only within the sandbox ceiling. Hard deny and sandbox deny always win over `AutoApprove`, `ApprovalMode=auto`, and rule allow.

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
	Resources   []ResourceDescriptor
	SupportsDryRun bool
	RateLimit   *RateLimitDescriptor
}
```

`AutoApprove` is a tool-level default, not an escape hatch. Sandbox hard-deny and explicit deny rules still win.

The descriptor must be registered before the tool is exposed to the model. Registration should validate:

- tool name is unique for the provider
- provider is known
- kind and risk are set
- external resources are declared for network, MCP, plugin, memory, and session tools
- destructive tools declare whether dry-run is supported
- tools with side effects or high cost declare rate-limit defaults
- tools without descriptors are rejected by the secure registry

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

Path handling requirements:

- Resolve the workspace root once at session startup using absolute path normalization and symlink evaluation.
- Path normalization order for existing paths is: convert to absolute path, reject unsupported Windows path forms, evaluate symlinks or reparse points, then clean and canonicalize for comparison.
- Path normalization order for new paths is: convert the requested target to an absolute lexical path, reject unsupported Windows path forms, walk existing parents from root to leaf, evaluate symlinks or reparse points for each existing parent, then clean and canonicalize the resolved parent plus final filename for comparison.
- For an existing target path, evaluate symlinks before containment checks.
- For a new target path, evaluate symlinks for every existing parent directory, then check the final path under the resolved parent.
- On Windows, normalize volume names, UNC roots, drive-letter case, path separators, and extended-length path prefixes before comparison.
- Reject or explicitly normalize Windows `\\?\` and `\\.\` device paths before containment checks. The first implementation should reject them unless there is a tested canonicalization path.
- Reject Windows reserved device names such as `CON`, `PRN`, `AUX`, `NUL`, `COM1` through `COM9`, and `LPT1` through `LPT9`, including names with extensions such as `COM1.txt`.
- Reject alternate data stream syntax such as `file.txt:stream` unless a future implementation has explicit ADS handling.
- Reject cross-volume and cross-drive escapes. A path whose resolved volume differs from the workspace volume is outside the workspace even if lexical cleanup appears safe.
- Reject paths that target filesystem roots, drive roots, home directory roots, or the workspace root for destructive operations.
- Treat Windows junctions and symlinks as escape-capable and test them explicitly.
- Do not follow a symlink after approval if the path can change between check and use. File tools should reopen and revalidate immediately before write/delete.
- Write/delete implementations should use no-follow or handle-based APIs where the platform supports them. On Unix-like systems prefer `O_NOFOLLOW` and directory file descriptors. On Windows prefer handle-based validation with final path checks after opening. If equivalent protection is not available, the operation must revalidate immediately before mutation and document the residual race risk.
- Prefer atomic writes for `replace_file`: write to a temporary file in the same directory, fsync where practical, then rename.
- `patch_file` must verify the patch applies to the expected original content before writing.
- File reads should enforce maximum byte limits and return `truncated=true` when the output is shortened.

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
- state-changing PowerShell cmdlets such as `Set-Content`, `Set-Item`, `New-Item`, `Copy-Item`, `Move-Item`, and `Rename-Item`
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

Command classification requirements:

- Use a three-layer classifier:
  1. shell-aware syntax parsing to detect command boundaries, quoting, chaining, redirection, pipeline, background execution, substitution, script blocks, and inline interpreter payloads
  2. executable allowlist or denylist classification for known commands such as `git status`, `git diff`, `go test`, package managers, shells, and network tools
  3. argument and output-risk classification to detect file writes, generated outputs, network access, privilege changes, destructive flags, and credential exposure
- Parse the command into executable and arguments before applying prefix rules. Do not match prefix rules against raw strings.
- Quoted operator-looking text must not by itself make a command complex. For example, `git commit -m "fix && test"` contains a quoted string, not shell chaining.
- Commands that write files must not be classified as read-only only because they contain no dangerous metacharacters. For example, `go build -o output ./...` is a write-producing command.
- Pipeline-to-shell patterns such as `curl ... | sh`, `iwr ... | iex`, and downloaded script execution must be high risk even if each individual executable is known.
- Prefix allow rules only apply to a single command invocation with no shell chaining, redirection, pipeline, background execution, command substitution, script block, or inline interpreter payload.
- Unknown commands are never auto-approved.
- Network-capable commands are at least `ask` by default, even when they look read-only.
- Package installation, deployment, upload, push, credential, and permission-changing commands are at least `ask`, and may be denied by default in `plan`.
- Command execution should pass environment variables through an allowlist or redacted snapshot for audit. Secret-looking environment values must not be logged.
- Command timeout must be enforced with context cancellation and process termination. Child process cleanup should be best effort and audited when cleanup fails.

Windows PowerShell-specific requirements:

- Reject auto-approval for `-EncodedCommand`, `-Command` containing script blocks, call operator `&`, dot-sourcing `.`, dynamic invocation, or aliases that cannot be resolved deterministically.
- Resolve common PowerShell aliases before classification, including `rm`, `del`, `erase`, `mv`, `cp`, `cat`, `curl`, `wget`, `iwr`, and `iex`.
- Treat `Invoke-*`, `Start-Process`, `New-Item`, `Remove-Item`, `Set-Item`, `Set-Content`, `Add-Content`, `Out-File`, `Copy-Item`, `Move-Item`, and `Rename-Item` as sensitive unless an explicit rule allows them.
- Detect PowerShell control operators and metacharacters, including `;`, `|`, `&&`, `||`, `>`, `>>`, `2>`, `*>`, `$()`, `@()`, backticks, here-strings, and script blocks.
- Do not auto-approve commands that invoke `powershell`, `pwsh`, `cmd`, `bash`, `wsl`, `python -c`, `node -e`, or another shell/interpreter with inline code.
- Prefer executing known simple commands without an extra shell when practical. If shell execution is required, classification must happen before shell invocation.

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

Sub-agent security requirements:

- Sub-agents inherit the parent `SecurityContext` and cannot raise `SandboxMode` or relax `ApprovalMode`.
- Sub-agent tool descriptors must be registered through the same secure registry.
- `create_agent` must validate name, description, instruction length, and permission mode before registration.
- `run_subagent` must record audit entries with parent session id, agent name, permission mode, task summary, duration, and result status.
- In-memory sub-agents should be treated as state mutation because they alter runtime behavior. In `/plan`, creation is allowed only for readonly/plan agents and should be auditable.

Sub-agent permission tool matrix:

| PermissionMode | Allowed tool surface | Notes |
| --- | --- | --- |
| `readonly` | `read_file`, `list_dir`, safe metadata tools, and known local read-only commands when parent sandbox allows command execution | No writes, deletes, network, external state mutation, or sub-agent creation. |
| `plan` | `readonly` surface plus `suggested` results for write/delete/command operations that would mutate state | Suggestions must not execute side effects. Useful for delegated research and implementation plans. |
| `default` | Parent session's secured tool surface, still bounded by parent `SecurityContext`, rules, approval, and sandbox | Cannot be created or run from `/plan` unless explicitly downgraded to `readonly` or `plan`. |

Tool filtering should happen when building the sub-agent runner, not only inside individual handlers. A sub-agent should never see tool names that its permission mode cannot use.

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

Default external-tool policy:

- Unknown external tools are denied until they register a valid descriptor.
- Tools with unknown resources or unknown destructive behavior are `ask` in `agent` and denied or suggested in `plan`.
- Network access requires declared domains or resource identifiers.
- Domain allowlists should match normalized hostnames, not raw URLs.
- Tools that can upload local files must run both file-read policy and external-resource policy.

### Network Policy

Network policy should be part of P0 data modeling even if enforcement is implemented later.

```go
type NetworkPolicy struct {
	DefaultDecision Decision
	AllowedDomains  []string
	DeniedDomains   []string
	AllowedPorts    []int
	DeniedPorts     []int
	AllowPrivateIPs bool
}
```

Rules:

- Default network decision is `ask` in `/agent`, `suggest` or `deny` in `/plan`, and `deny` in `/ask`.
- Domain matching must use normalized hostnames after URL parsing and IDNA handling.
- Raw IPs, private IP ranges, localhost, link-local addresses, and metadata service IPs are denied unless explicitly allowed.
- A command or MCP/plugin tool that can access the network must declare its target domains or mark them unknown. Unknown network targets are never auto-approved.
- Upload operations combine network policy with file-read policy. Sensitive local files remain denied even when the destination domain is allowed.

### Rate Limits And Backoff

The security layer should prevent accidental or model-induced loops from repeatedly invoking the same tool.

Recommended counters:

- per-tool calls per session
- per-tool calls per minute
- per-session total tool calls
- per-session total command runtime
- per-session total output bytes returned to the model
- repeated identical denial count

When limits are exceeded, tools should return `status="denied"` or `status="rate_limited"` with a short reason and a retry hint where appropriate. Repeated identical blocked operations should trigger exponential backoff or a session-level stop condition so the model does not keep retrying the same denied action.

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
- `rate_limited`

This lets the model revise behavior instead of retrying the same blocked action.

Recommended generic envelope:

```go
type ToolResult[T any] struct {
	OK         bool   `json:"ok"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
	Data       T      `json:"data,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
	AuditID    string `json:"audit_id,omitempty"`
}
```

Use Go `error` for infrastructure failures that prevent a valid tool result from being produced, such as JSON schema errors, handler panics, broken audit sink initialization, or model/runtime cancellation. Use `ToolResult` statuses for policy denials, approval requirements, user rejection, command failure, timeout, and truncated output. This avoids teaching the model that policy denials are transient runtime errors.

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

Additional default secret targets:

- cloud credentials such as AWS, Azure, GCP, and OCI credential files
- Kubernetes config and kubeconfig files
- `.npmrc`, `.pypirc`, `.netrc`, package registry tokens, and deploy keys
- GitHub, GitLab, npm, Docker, OpenAI, Anthropic, and other API token files
- password stores, browser profile credential stores, and SSH config containing host secrets

Hard-deny checks should distinguish between reading file contents, listing filenames, writing files, deleting files, and uploading files. Some paths may be allowed for existence checks but denied for content reads.

### Data Directory Access

`~/eino` is a data directory, not a workspace. It needs its own policy:

- session metadata may be read by session tools, not by general file tools unless explicitly allowed
- audit logs are append-only for the audit sink and should not be editable through file tools
- memory files should be readable only through memory tools that apply their own policy and redaction
- credential, token, key, and secret subdirectories under `~/eino` are hard-denied by default
- project rules in `.eino` may be visible to file tools only when they are inside the workspace and pass normal containment checks

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

Audit safety requirements:

- Audit records must redact secret-looking argument values, environment values, headers, URLs with credentials, and file contents.
- Store argument summaries instead of full arguments for large inputs or sensitive tools.
- Include a stable audit id in every structured tool result.
- The audit sink must be append-only from the security layer's perspective.
- Audit failures should be visible. For local interactive sessions they may fail closed for high-risk operations; for low-risk read operations they may return a structured warning according to configuration.

## Rules Engine Roadmap

The first implementation can use built-in policy. The next stage should add declarative rules inspired by Codex-style exec policy and Claude-style allow/ask/deny rules.

Candidate locations:

- `.eino/rules.star` for project policy
- `~/eino/rules.star` for user policy

Rule examples:

```python
prefix_rule(pattern = ["git", "status"], operation = "exec", decision = "allow")
prefix_rule(pattern = ["git", "push"], operation = "exec", decision = "ask")
glob_rule(pattern = "*.env", operation = "read", decision = "deny")
glob_rule(pattern = "*.env", operation = "upload", decision = "deny")
tool_rule(tool = "delete_file", operation = "delete", decision = "ask")
when(session_mode = "plan", tool = "replace_file", operation = "write", decision = "deny")
```

Evaluation order:

1. hard deny
2. sandbox deny
3. explicit rule deny
4. explicit rule ask
5. explicit rule allow
6. built-in default policy

Rules should support hot reload. If reload fails, keep the last valid rule set, report the parse error, and deny newly requested high-risk operations until a valid rule set is loaded again.

Rule engine requirements:

- Rule files must be parsed and validated before activation.
- If hot reload fails, keep the last valid rule set and add an audit entry for the reload failure.
- Project rules may further restrict user rules, but should not silently expand beyond the active sandbox ceiling.
- User rules and project rules need deterministic precedence. Recommended order: hard deny, sandbox deny, project deny, user deny, project ask, user ask, project allow, user allow, built-in default.
- Glob rules must run on normalized workspace-relative paths after containment checks.
- Prefix command rules must run on parsed command tokens, not raw command text.
- Rules must declare an `operation` such as `read`, `list`, `write`, `delete`, `exec`, `network`, `upload`, `memory_write`, or `external_state`. A path rule without operation is invalid.

### Rule DSL Selection

Do not lock the implementation to one DSL before P2. `.eino/rules.star` and `~/eino/rules.star` are candidate paths, not final API.

P2 should compare at least:

- Starlark: expressive, deterministic subset of Python-like syntax, good for custom policy helpers, but larger attack surface than simple expressions.
- CEL: lightweight expression language, good embedding story, smaller policy surface, less convenient for custom rule declarations.
- Rego/OPA: mature policy ecosystem and strong semantics, but heavier operational and dependency footprint for a CLI.
- HCL or structured config: easy to read and may align with existing config tooling, but less expressive for conditional policy.

Initial recommendation: start P2 with a small structured rule model in Go and add a DSL only after the required rule shapes are stable. If a DSL is introduced, keep the Go rule AST as the internal representation so DSL changes do not affect policy evaluation.

## Testing Strategy

P0 and P1 must include table-driven tests and adversarial cases before broad tool rollout.

Path boundary tests:

- `..` lexical escapes
- absolute path escapes
- cross-drive and cross-volume paths
- UNC paths
- `\\?\` and `\\.\` Windows device paths
- NTFS junction escapes
- symlink escapes for existing files
- symlink parent escapes for new files
- Windows case variants
- reserved names such as `CON`, `NUL`, and `COM1.txt`
- alternate data stream syntax
- missing target creation under safe and unsafe parents
- destructive operation attempts against workspace root, home root, drive root, and filesystem root

Mode matrix tests:

- table-driven coverage for all relevant `(SessionMode, SandboxMode, ApprovalMode, ToolKind, OperationRisk)` combinations
- explicit assertions for `ask`, `plan`, and `agent` defaults
- assertions that `danger-full-access` still preserves hard-deny protections

Command classifier tests:

- quoted operators such as `git commit -m "fix && test"` are not treated as shell chaining
- `git status; rm -rf .`, pipelines, redirects, substitutions, and script blocks are not prefix-allowed
- write-producing commands such as `go build -o output ./...` are not read-only
- network-to-shell patterns such as `curl ... | sh` and `iwr ... | iex` are high risk
- PowerShell aliases are resolved before classification
- `-EncodedCommand`, inline interpreters, and nested shells are not auto-approved
- fuzz tests for random command input must never panic and must fail closed for unknown parse shapes

Rule tests:

- deny-first precedence is stable
- project and user rule precedence is deterministic
- invalid rules do not replace the last valid rule set
- high-risk operations are denied while rule reload is in an error state
- glob rules apply only to their declared operation kind

Runtime integration tests:

- unregistered tools cannot be exposed through the Eino adapter
- sub-agent permission modes expose only the allowed tool surface
- audit records include session id and audit id
- audit redaction removes secrets from arguments, URLs, headers, and environment summaries
- rate limits return structured `rate_limited` results and stop repeated identical denied calls

## Implementation Roadmap

### P0: Sandbox Boundary And Decision Model

- Add `internal/security`.
- Define sandbox modes, tool kinds, operation risks, decisions, and operation metadata.
- Define immutable `SecurityContext` and validate legal mode combinations.
- Define `ToolDescriptor`, `OperationRequest`, `PolicyDecision`, and generic `ToolResult`.
- Implement workspace root normalization and containment checks.
- Reject path escapes and symlink escapes.
- Implement hard-deny policy for destructive roots, `.git` internals, and known secrets.
- Implement secure registry/wrapper interfaces without full built-in file or command tools yet.
- Add unit tests for path boundaries and mode matrix.

### P1: Approval And Built-In Tools

- Add `internal/approval`.
- Implement CLI prompter and fake prompter tests.
- Add file tools: read, list, create, patch, replace, delete.
- Add command tool with conservative classifier.
- Route all built-in tools through sandbox and approval.
- Add structured tool results.
- Add audit log interface.
- Replace direct Eino tool registration with secure wrappers.
- Add tests proving unregistered tools cannot be exposed to the model.

### P2: Declarative Rule Engine

- Add rules file loading.
- Implement prefix, glob, tool, and session-mode rules.
- Enforce deny-first evaluation.
- Add sample rules.
- Add tests for precedence and reload failures.
- Add tests proving prefix rules do not match chained, piped, redirected, substituted, or aliased commands.

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
- The Eino runtime only receives tools from `SecureToolRegistry`; direct raw tool handlers are not registered.
- `SecurityContext` has a tested legal combination matrix.
- Path tests cover `..`, absolute escapes, symlinks, junctions, missing targets, drive roots, UNC paths, and case normalization.
- Command tests cover PowerShell aliases, `-EncodedCommand`, inline interpreters, redirection, pipelines, and shell chaining.
- Audit records redact secrets and include stable audit ids.
- Unknown tools, unknown command shapes, and unknown external resources fail closed.
