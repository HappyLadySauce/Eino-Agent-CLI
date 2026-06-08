package tools

import (
	"context"
	"os"

	einotool "github.com/cloudwego/eino/components/tool"

	agenttools "github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/agent"
	commandtools "github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/command"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"
	filetools "github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/file"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/approval"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/audit"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/rules"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
)

// SecureToolOptions contains shared dependencies for secured tools.
// SecureToolOptions 包含安全工具的共享依赖。
type SecureToolOptions = core.SecureToolOptions

// AgentToolService is the runtime surface used by agent orchestration tools.
// AgentToolService 是 Agent 编排工具依赖的运行时服务面。
type AgentToolService = agenttools.Service

// CreateAgentInput is the schema for creating a dynamic sub-agent.
// CreateAgentInput 是创建动态子 Agent 的工具入参。
type CreateAgentInput = agenttools.CreateAgentInput

// CreateAgentOutput is returned after a dynamic sub-agent is registered.
// CreateAgentOutput 是动态子 Agent 注册后的返回值。
type CreateAgentOutput = agenttools.CreateAgentOutput

// RunSubAgentInput is the schema for running a sub-agent with isolated context.
// RunSubAgentInput 是使用隔离上下文运行子 Agent 的工具入参。
type RunSubAgentInput = agenttools.RunSubAgentInput

// RunSubAgentOutput contains only the final sub-agent result.
// RunSubAgentOutput 只包含子 Agent 的最终结果。
type RunSubAgentOutput = agenttools.RunSubAgentOutput

// ListAgentsInput is intentionally empty because listing needs no arguments.
// ListAgentsInput 故意为空，因为列出 Agent 不需要参数。
type ListAgentsInput = agenttools.ListAgentsInput

// AgentSummary is a compact description of one registered agent.
// AgentSummary 是一个已注册 Agent 的紧凑描述。
type AgentSummary = agenttools.AgentSummary

// ListAgentsOutput returns available agents to the main agent.
// ListAgentsOutput 向主 Agent 返回可用 Agent。
type ListAgentsOutput = agenttools.ListAgentsOutput

// NewAgentTools creates all main-agent tools for the given session mode.
// NewAgentTools 为指定会话模式创建主 Agent 工具集合。
func NewAgentTools(mode string, service AgentToolService) ([]einotool.BaseTool, error) {
	workspace, _ := os.Getwd()
	secCtx, err := security.DefaultContextForSession("test-session", workspace, core.DefaultDataDir(), core.ToSecuritySessionMode(mode))
	if err != nil {
		return nil, err
	}
	return NewSecureAgentTools(mode, service, SecureToolOptions{
		Context:     secCtx,
		Prompter:    &approval.FakePrompter{Decision: approval.DecisionDeny},
		AuditSink:   audit.NewMemorySink(),
		RuleSet:     rules.NewSet(),
		RateLimiter: security.NewRateLimiter(),
	})
}

// NewSecureAgentTools creates secured tools for a mode.
// NewSecureAgentTools 为指定模式创建安全工具。
func NewSecureAgentTools(mode string, service AgentToolService, opts SecureToolOptions) ([]einotool.BaseTool, error) {
	definitions := make([]core.Definition, 0, 16)
	definitions = append(definitions, agenttools.Definitions()...)
	definitions = append(definitions, filetools.Definitions()...)
	definitions = append(definitions, commandtools.Definitions()...)
	return core.BuildDefinitions(core.Runtime{
		Mode:    mode,
		Service: service,
		Options: opts.WithDefaults(),
	}, definitions)
}

// Compile-time service check documents the root facade contract.
// 编译期服务检查用于记录根门面契约。
var _ AgentToolService = agentToolService(nil)

type agentToolService interface {
	ListAgents(context.Context, string, ListAgentsInput) (*ListAgentsOutput, error)
	CreateAgent(context.Context, string, CreateAgentInput) (*CreateAgentOutput, error)
	RunSubAgent(context.Context, string, RunSubAgentInput) (*RunSubAgentOutput, error)
}
