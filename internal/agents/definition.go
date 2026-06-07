package agents

import (
	"sort"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/prompts"
)

const (
	AgentGeneralPurpose = "general-purpose"
	AgentExplore        = "explore"
	AgentPlan           = "plan"
	AgentVerify         = "verify"
)

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

// BuiltinAgentDefinitions returns the built-in agent roles.
// BuiltinAgentDefinitions 返回内置 Agent 角色。
func BuiltinAgentDefinitions() []AgentDefinition {
	definitions := []AgentDefinition{
		{
			Name:           AgentGeneralPurpose,
			Description:    "General-purpose Chinese CLI assistant that can coordinate specialized sub-agents when useful.",
			SystemPrompt:   prompts.GeneralPurposeAgentInstruction,
			PermissionMode: PermissionModeDefault,
		},
		{
			Name:           AgentExplore,
			Description:    "Read-only exploration agent for understanding code, architecture, errors, and documentation.",
			SystemPrompt:   prompts.ExploreAgentInstruction,
			PermissionMode: PermissionModeReadonly,
		},
		{
			Name:           AgentPlan,
			Description:    "Planning agent for producing implementation plans, interfaces, data flow, risks, and tests.",
			SystemPrompt:   prompts.PlanAgentInstruction,
			PermissionMode: PermissionModePlan,
		},
		{
			Name:           AgentVerify,
			Description:    "Verification agent for reviewing results, test strategy, failure modes, and acceptance criteria.",
			SystemPrompt:   prompts.VerifyAgentInstruction,
			PermissionMode: PermissionModeReadonly,
		},
	}

	sort.Slice(definitions, func(i, j int) bool {
		return definitions[i].Name < definitions[j].Name
	})
	return definitions
}
