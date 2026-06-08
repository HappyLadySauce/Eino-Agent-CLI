package agents

// PermissionMode describes the intended safety boundary of an agent.
// PermissionMode 描述 Agent 的预期安全边界。
type PermissionMode string

const (
	PermissionModeDefault  PermissionMode = "default"
	PermissionModeReadonly PermissionMode = "readonly"
	PermissionModePlan     PermissionMode = "plan"
)

// IsSubAgentSafeInPlan reports whether an agent may run in Plan mode.
// IsSubAgentSafeInPlan 判断 Agent 是否允许在 Plan 模式下运行。
func (m PermissionMode) IsSubAgentSafeInPlan() bool {
	return m == PermissionModeReadonly || m == PermissionModePlan
}

// AgentDefinition describes one runnable agent role.
// AgentDefinition 描述一个可运行的 Agent 角色。
type AgentDefinition struct {
	Name           string
	Description    string
	SystemPrompt   string
	PermissionMode PermissionMode
	Dynamic        bool
}
