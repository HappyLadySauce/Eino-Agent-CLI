package prompts

const (
	ModeAgents = "agents"
	ModePlan   = "plan"
	ModeAsk    = "ask"
)

// MainAgentInstruction returns the system prompt for a CLI session mode.
// MainAgentInstruction 返回 CLI 会话模式对应的系统提示词。
func MainAgentInstruction(mode string) string {
	switch mode {
	case ModePlan:
		return `你是 Eino-Agent-CLI Agent。请始终使用中文回答。
你的目标是产出可执行的技术方案，不执行代码修改。
你可以调用查找资料、验证假设、拆解方案。`
	case ModeAsk:
		return `你是 Eino-Agent-CLI 的 Ask 模式主 Agent。请始终使用中文回答。
你只回答问题、解释代码和澄清概念。
不要创建 agent，不要调用 subagent，不要声称已经执行任务或修改文件。`
	default:
		return `你是 Eino-Agent-CLI Agent。请始终使用中文回答。
你可以自主调用完成用户任务。`
	}
}

// SubAgentPrompt builds the isolated prompt passed to a sub-agent.
// SubAgentPrompt 构造传给子 Agent 的隔离提示词。
func SubAgentPrompt(task, contextSummary, expectedOutput string) string {
	prompt := "Task:\n" + task
	if contextSummary != "" {
		prompt += "\n\nContext summary from main agent:\n" + contextSummary
	}
	if expectedOutput != "" {
		prompt += "\n\nExpected output:\n" + expectedOutput
	}
	return prompt
}
