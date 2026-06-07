package tools

// CreateAgentInput is the schema for creating a dynamic sub-agent.
// CreateAgentInput 是创建动态子 Agent 的工具入参。
type CreateAgentInput struct {
	Name           string `json:"name" jsonschema:"required" jsonschema_description:"Unique lowercase agent name using letters, digits, and hyphens."`
	Description    string `json:"description" jsonschema:"required" jsonschema_description:"Short description of when this agent should be used."`
	Instruction    string `json:"instruction" jsonschema:"required" jsonschema_description:"System instruction for the new sub-agent."`
	PermissionMode string `json:"permission_mode,omitempty" jsonschema_description:"Permission mode: readonly, plan, or default. In plan mode only readonly and plan are allowed."`
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
	AgentName      string `json:"agent_name,omitempty" jsonschema_description:"Optional existing or newly created agent name. Empty uses a safe default for the current mode."`
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
