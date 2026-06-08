package tools

// CreateAgentInput is the schema for creating a dynamic sub-agent.
// CreateAgentInput 是创建动态子 Agent 的工具入参。
type CreateAgentInput struct {
	Name           string `json:"name" jsonschema:"required" jsonschema_description:"Unique lowercase agent name using letters, digits, and hyphens."`
	Description    string `json:"description" jsonschema:"required" jsonschema_description:"Short description of when this agent should be used."`
	Instruction    string `json:"instruction" jsonschema:"required" jsonschema_description:"System instruction for the new sub-agent."`
	PermissionMode string `json:"permission_mode" jsonschema:"required" jsonschema_description:"Required permission mode: readonly, plan, or default. In plan mode only readonly and plan are allowed."`
}

// CreateAgentOutput is returned after a dynamic sub-agent is registered.
// CreateAgentOutput 是动态子 Agent 注册后的返回值。
type CreateAgentOutput struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	PermissionMode string `json:"permission_mode"`
	Created        bool   `json:"created"`
}

// RunSubAgentInput is the schema for running a sub-agent with isolated context.
// RunSubAgentInput 是使用隔离上下文运行子 Agent 的工具入参。
type RunSubAgentInput struct {
	Task           string `json:"task" jsonschema:"required" jsonschema_description:"Complete task for the sub-agent."`
	AgentName      string `json:"agent_name" jsonschema:"required" jsonschema_description:"Required existing or newly created agent name. There is no default sub-agent."`
	ContextSummary string `json:"context_summary,omitempty" jsonschema_description:"Only the necessary background summarized by the main agent; never pass full chat history."`
	ExpectedOutput string `json:"expected_output,omitempty" jsonschema_description:"Optional expected output format or acceptance criteria."`
}

// RunSubAgentOutput contains only the final sub-agent result.
// RunSubAgentOutput 只包含子 Agent 的最终结果。
type RunSubAgentOutput struct {
	AgentName  string `json:"agent_name"`
	Content    string `json:"content"`
	Created    bool   `json:"created"`
	Duration   string `json:"duration"`
	EventCount int    `json:"event_count"`
	ChunkCount int    `json:"chunk_count"`
}

// ListAgentsInput is intentionally empty because listing needs no arguments.
// ListAgentsInput 故意为空，因为列出 Agent 不需要参数。
type ListAgentsInput struct{}

// AgentSummary is a compact description of one registered agent.
// AgentSummary 是一个已注册 Agent 的紧凑描述。
type AgentSummary struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	PermissionMode string `json:"permission_mode"`
	Dynamic        bool   `json:"dynamic"`
}

// ListAgentsOutput returns available agents to the main agent.
// ListAgentsOutput 向主 Agent 返回可用 Agent。
type ListAgentsOutput struct {
	Agents []AgentSummary `json:"agents"`
}

// ReadFileInput is the schema for reading a workspace file.
// ReadFileInput 是读取工作区文件的工具入参。
type ReadFileInput struct {
	Path     string `json:"path" jsonschema:"required" jsonschema_description:"Workspace-relative file path to read."`
	MaxBytes int64  `json:"max_bytes,omitempty" jsonschema_description:"Optional maximum bytes to return."`
}

// ReadFileOutput contains file content and truncation metadata.
// ReadFileOutput 包含文件内容和截断信息。
type ReadFileOutput struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

// ListDirInput is the schema for listing a workspace directory.
// ListDirInput 是列出工作区目录的工具入参。
type ListDirInput struct {
	Path string `json:"path" jsonschema:"required" jsonschema_description:"Workspace-relative directory path to list."`
}

// DirEntry is a compact directory entry.
// DirEntry 是紧凑的目录项。
type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// ListDirOutput contains directory entries.
// ListDirOutput 包含目录项。
type ListDirOutput struct {
	Path    string     `json:"path"`
	Entries []DirEntry `json:"entries"`
}

// WriteFileInput is shared by create and replace file tools.
// WriteFileInput 是创建与替换文件工具的共享入参。
type WriteFileInput struct {
	Path    string `json:"path" jsonschema:"required" jsonschema_description:"Workspace-relative file path."`
	Content string `json:"content" jsonschema:"required" jsonschema_description:"Complete UTF-8 file content."`
	DryRun  bool   `json:"dry_run,omitempty" jsonschema_description:"When true, describe the change without writing."`
}

// PatchFileInput applies a simple full-content guarded patch.
// PatchFileInput 应用带原内容保护的简单补丁。
type PatchFileInput struct {
	Path    string `json:"path" jsonschema:"required"`
	OldText string `json:"old_text" jsonschema:"required" jsonschema_description:"Text expected to exist exactly once."`
	NewText string `json:"new_text" jsonschema:"required" jsonschema_description:"Replacement text."`
	DryRun  bool   `json:"dry_run,omitempty"`
}

// DeleteFileInput deletes one workspace file.
// DeleteFileInput 删除一个工作区文件。
type DeleteFileInput struct {
	Path   string `json:"path" jsonschema:"required"`
	DryRun bool   `json:"dry_run,omitempty"`
}

// FileMutationOutput describes a file mutation result.
// FileMutationOutput 描述文件变更结果。
type FileMutationOutput struct {
	Path    string `json:"path"`
	Changed bool   `json:"changed"`
	Message string `json:"message"`
}

// RunCommandInput runs a local command.
// RunCommandInput 运行本地命令。
type RunCommandInput struct {
	Command        string `json:"command" jsonschema:"required"`
	CWD            string `json:"cwd,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	DryRun         bool   `json:"dry_run,omitempty"`
}

// RunCommandOutput contains command execution results.
// RunCommandOutput 包含命令执行结果。
type RunCommandOutput struct {
	Command   string `json:"command"`
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Truncated bool   `json:"truncated"`
	Risk      string `json:"risk"`
	DryRun    bool   `json:"dry_run"`
}

// MemoryInput targets one memory entry.
// MemoryInput 定位一条记忆。
type MemoryInput struct {
	Name    string `json:"name" jsonschema:"required"`
	Content string `json:"content,omitempty"`
	DryRun  bool   `json:"dry_run,omitempty"`
}

// MemoryOutput describes memory operation result.
// MemoryOutput 描述记忆操作结果。
type MemoryOutput struct {
	Name    string   `json:"name,omitempty"`
	Content string   `json:"content,omitempty"`
	Names   []string `json:"names,omitempty"`
	Changed bool     `json:"changed,omitempty"`
	Message string   `json:"message,omitempty"`
}

// SessionOutput summarizes the current security session.
// SessionOutput 汇总当前安全会话。
type SessionOutput struct {
	SessionID string `json:"session_id"`
	Mode      string `json:"mode"`
	Sandbox   string `json:"sandbox"`
	Approval  string `json:"approval"`
}

// SessionMetadata is persisted non-sensitive session metadata.
// SessionMetadata 是持久化的非敏感会话元数据。
type SessionMetadata struct {
	SessionID string `json:"session_id"`
	Mode      string `json:"mode"`
	Sandbox   string `json:"sandbox"`
	Approval  string `json:"approval"`
	CreatedAt string `json:"created_at"`
}
