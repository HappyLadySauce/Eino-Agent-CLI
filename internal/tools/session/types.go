package session

// EmptyInput is used by tools that need no arguments.
// EmptyInput 用于不需要参数的工具。
type EmptyInput struct{}

// MemoryInput identifies and optionally writes one memory entry.
// MemoryInput 标识并可选写入一条 memory。
type MemoryInput struct {
	Name    string `json:"name" jsonschema:"required"`
	Content string `json:"content,omitempty"`
	DryRun  bool   `json:"dry_run,omitempty"`
}

// MemoryOutput contains memory read/write/list results.
// MemoryOutput 包含 memory 读写列表结果。
type MemoryOutput struct {
	Name    string   `json:"name,omitempty"`
	Names   []string `json:"names,omitempty"`
	Content string   `json:"content,omitempty"`
	Changed bool     `json:"changed,omitempty"`
	Message string   `json:"message,omitempty"`
}

// InfoOutput returns current security session metadata.
// InfoOutput 返回当前安全会话元数据。
type InfoOutput struct {
	SessionID string `json:"session_id"`
	Mode      string `json:"mode"`
	Sandbox   string `json:"sandbox"`
	Approval  string `json:"approval"`
}

// Metadata is the persisted minimal session metadata.
// Metadata 是持久化的最小会话元数据。
type Metadata struct {
	SessionID string `json:"session_id"`
	Mode      string `json:"mode"`
	Sandbox   string `json:"sandbox"`
	Approval  string `json:"approval"`
	CreatedAt string `json:"created_at"`
}
