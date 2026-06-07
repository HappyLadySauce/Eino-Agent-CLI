package agents

import "sort"

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

// AgentDefinition describes one runnable agent role.
// AgentDefinition 描述一个可运行的 Agent 角色。
type AgentDefinition struct {
	Name             string
	Description      string
	SystemPrompt     string
	PermissionMode   PermissionMode
	CanRunInParallel bool
}

// BuiltinAgentDefinitions returns the built-in agent roles.
// BuiltinAgentDefinitions 返回内置 Agent 角色。
func BuiltinAgentDefinitions() []AgentDefinition {
	definitions := []AgentDefinition{
		{
			Name:        AgentGeneralPurpose,
			Description: "General-purpose Chinese CLI assistant that can coordinate specialized sub-agents when useful.",
			SystemPrompt: `你是 Eino-Agent-CLI 的主 Agent。请始终使用中文回答用户。
你可以在需要时调用 explore、plan、verify 子 Agent 工具。
不要声称已经修改文件，除非工具或用户明确提供了对应事实。
日志文本和面向机器的状态信息必须使用英文。`,
			PermissionMode:   PermissionModeDefault,
			CanRunInParallel: false,
		},
		{
			Name:        AgentExplore,
			Description: "Read-only exploration agent for understanding code, architecture, errors, and documentation.",
			SystemPrompt: `你是只读分析 Agent。请始终使用中文回答。
只做事实梳理、代码阅读结论、风险识别和下一步建议。
不要提出会修改文件的结论，不要假装已经执行写操作。`,
			PermissionMode:   PermissionModeReadonly,
			CanRunInParallel: true,
		},
		{
			Name:        AgentPlan,
			Description: "Planning agent for producing implementation plans, interfaces, data flow, risks, and tests.",
			SystemPrompt: `你是规划 Agent。请始终使用中文回答。
输出可执行的技术方案，覆盖模块划分、接口、数据流、错误处理、测试与风险。
只产出计划，不执行代码修改。`,
			PermissionMode:   PermissionModePlan,
			CanRunInParallel: true,
		},
		{
			Name:        AgentVerify,
			Description: "Verification agent for reviewing results, test strategy, failure modes, and acceptance criteria.",
			SystemPrompt: `你是验证 Agent。请始终使用中文回答。
重点检查错误传递、context 传递、边界条件、测试覆盖、并发安全和验收标准。
只输出验证结论和改进建议。`,
			PermissionMode:   PermissionModeReadonly,
			CanRunInParallel: true,
		},
	}

	sort.Slice(definitions, func(i, j int) bool {
		return definitions[i].Name < definitions[j].Name
	})
	return definitions
}
