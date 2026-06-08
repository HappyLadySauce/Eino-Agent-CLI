package tools

import (
	"context"
	"fmt"

	einotool "github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
)

// AgentToolService is the runtime surface used by agent orchestration tools.
// AgentToolService 是 Agent 编排工具依赖的运行时服务面。
type AgentToolService interface {
	ListAgents(ctx context.Context, mode string, input ListAgentsInput) (*ListAgentsOutput, error)
	CreateAgent(ctx context.Context, mode string, input CreateAgentInput) (*CreateAgentOutput, error)
	RunSubAgent(ctx context.Context, mode string, input RunSubAgentInput) (*RunSubAgentOutput, error)
}

// NewAgentTools creates all main-agent tools for the given session mode.
// NewAgentTools 为指定会话模式创建主 Agent 工具集合。
func NewAgentTools(mode string, service AgentToolService) ([]einotool.BaseTool, error) {
	listAgentsTool, err := toolutils.InferTool[ListAgentsInput, *ListAgentsOutput](
		"list_agents",
		"List available dynamically created sub-agents with their permissions. The list may be empty; use this before deciding whether to reuse an agent or create a new one.",
		func(ctx context.Context, input ListAgentsInput) (*ListAgentsOutput, error) {
			return service.ListAgents(ctx, mode, input)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create list_agents tool: %w", err)
	}

	createAgentTool, err := toolutils.InferTool[CreateAgentInput, *CreateAgentOutput](
		"create_agent",
		"Create a new in-memory sub-agent when existing agents are not specific enough. permission_mode is required. In plan mode only readonly or plan agents are allowed.",
		func(ctx context.Context, input CreateAgentInput) (*CreateAgentOutput, error) {
			return service.CreateAgent(ctx, mode, input)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create create_agent tool: %w", err)
	}

	runSubAgentTool, err := toolutils.InferTool[RunSubAgentInput, *RunSubAgentOutput](
		"run_subagent",
		"Run one named sub-agent with a fresh isolated context. agent_name is required. Provide task and only the necessary context_summary; the final result is returned to the main agent.",
		func(ctx context.Context, input RunSubAgentInput) (*RunSubAgentOutput, error) {
			return service.RunSubAgent(ctx, mode, input)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create run_subagent tool: %w", err)
	}

	return []einotool.BaseTool{listAgentsTool, createAgentTool, runSubAgentTool}, nil
}
