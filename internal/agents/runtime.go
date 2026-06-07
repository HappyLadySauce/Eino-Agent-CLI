package agents

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/messages"
	"github.com/HappyLadySauce/Eino-Agent-CLI/pkg/config"
)

// AgentRuntime owns ADK agents, runners, and orchestration settings.
// AgentRuntime 持有 ADK Agent、Runner 与编排设置。
type AgentRuntime struct {
	registry           *AgentRegistry
	runners            map[string]*adk.Runner
	maxHistoryMessages int
	maxParallelWorkers int
}

// NewAgentRuntime creates the model, sub-agents, main agent, and runners.
// NewAgentRuntime 创建模型、子 Agent、主 Agent 与 Runner。
func NewAgentRuntime(ctx context.Context, cfg *config.Config) (*AgentRuntime, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if cfg == nil || cfg.Model == nil {
		return nil, errors.New("agent config is missing model settings")
	}
	if cfg.Agent == nil {
		return nil, errors.New("agent config is missing agent settings")
	}
	if cfg.Agent.MaxParallelWorkers < 1 {
		return nil, fmt.Errorf("max_parallel_workers must be greater than or equal to 1, got %d", cfg.Agent.MaxParallelWorkers)
	}

	model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  cfg.Model.AuthToken,
		BaseURL: cfg.Model.BaseURL,
		Model:   cfg.Model.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("create chat model: %w", err)
	}

	registry, err := NewAgentRegistry(BuiltinAgentDefinitions())
	if err != nil {
		return nil, fmt.Errorf("create agent registry: %w", err)
	}

	createdAgents := make(map[string]*adk.ChatModelAgent, len(registry.List()))
	for _, definition := range registry.List() {
		if definition.Name == AgentGeneralPurpose {
			continue
		}
		agent, err := newChatModelAgent(ctx, model, definition, nil)
		if err != nil {
			return nil, fmt.Errorf("create sub-agent %q: %w", definition.Name, err)
		}
		createdAgents[definition.Name] = agent
	}

	tools := make([]tool.BaseTool, 0, len(createdAgents))
	for _, name := range []string{AgentExplore, AgentPlan, AgentVerify} {
		agent := createdAgents[name]
		if agent == nil {
			continue
		}
		tools = append(tools, adk.NewAgentTool(ctx, agent))
	}

	mainDefinition, err := registry.Get(AgentGeneralPurpose)
	if err != nil {
		return nil, err
	}
	mainAgent, err := newChatModelAgent(ctx, model, mainDefinition, tools)
	if err != nil {
		return nil, fmt.Errorf("create main agent: %w", err)
	}
	createdAgents[AgentGeneralPurpose] = mainAgent

	runners := make(map[string]*adk.Runner, len(createdAgents))
	for name, agent := range createdAgents {
		runners[name] = adk.NewRunner(ctx, adk.RunnerConfig{
			Agent:           agent,
			EnableStreaming: true,
		})
	}

	return &AgentRuntime{
		registry:           registry,
		runners:            runners,
		maxHistoryMessages: cfg.Model.MaxOutputTokens,
		maxParallelWorkers: cfg.Agent.MaxParallelWorkers,
	}, nil
}

// Definitions returns all registered agent definitions.
// Definitions 返回所有已注册 Agent 定义。
func (r *AgentRuntime) Definitions() []AgentDefinition {
	if r == nil {
		return nil
	}
	return r.registry.List()
}

// MaxHistoryMessages returns the configured conversation history limit.
// MaxHistoryMessages 返回配置的对话历史限制。
func (r *AgentRuntime) MaxHistoryMessages() int {
	if r == nil {
		return messages.DefaultMaxCount
	}
	return r.maxHistoryMessages
}

// MaxParallelWorkers returns the configured worker concurrency limit.
// MaxParallelWorkers 返回配置的 worker 并发上限。
func (r *AgentRuntime) MaxParallelWorkers() int {
	if r == nil || r.maxParallelWorkers <= 0 {
		return 1
	}
	return r.maxParallelWorkers
}

// RunMain runs the main agent with shared conversation history.
// RunMain 使用共享对话历史运行主 Agent。
func (r *AgentRuntime) RunMain(ctx context.Context, history []*schema.Message, writer io.Writer) (*messages.AssistantStreamResult, error) {
	return r.run(ctx, AgentGeneralPurpose, history, writer)
}

// RunSync runs one agent with isolated conversation history.
// RunSync 使用隔离对话历史运行一个 Agent。
func (r *AgentRuntime) RunSync(ctx context.Context, agentName, prompt string, writer io.Writer) (*messages.AssistantStreamResult, error) {
	if prompt == "" {
		return nil, errors.New("agent prompt is required")
	}
	result, err := r.run(ctx, agentName, []*schema.Message{schema.UserMessage(prompt)}, writer)
	if err != nil {
		return nil, fmt.Errorf("run sub-agent %q: %w", agentName, err)
	}
	return result, nil
}

func (r *AgentRuntime) run(ctx context.Context, agentName string, history []*schema.Message, writer io.Writer) (*messages.AssistantStreamResult, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if r == nil {
		return nil, errors.New("agent runtime is nil")
	}
	if _, err := r.registry.Get(agentName); err != nil {
		return nil, err
	}
	runner := r.runners[agentName]
	if runner == nil {
		return nil, fmt.Errorf("agent runner %q is not initialized", agentName)
	}
	iter := runner.Run(ctx, history)
	result, err := messages.ConsumeAssistantStream(iter, writer)
	if err != nil {
		return nil, fmt.Errorf("consume assistant stream for agent %q: %w", agentName, err)
	}
	return result, nil
}

func newChatModelAgent(ctx context.Context, model *openai.ChatModel, definition AgentDefinition, tools []tool.BaseTool) (*adk.ChatModelAgent, error) {
	if definition.Name == "" {
		return nil, errors.New("agent name is required")
	}
	if definition.Description == "" {
		return nil, fmt.Errorf("agent %q description is required", definition.Name)
	}

	config := &adk.ChatModelAgentConfig{
		Name:        definition.Name,
		Description: definition.Description,
		Model:       model,
		Instruction: definition.SystemPrompt,
	}
	if len(tools) > 0 {
		config.ToolsConfig = adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
			EmitInternalEvents: true,
		}
	}
	agent, err := adk.NewChatModelAgent(ctx, config)
	if err != nil {
		return nil, err
	}
	return agent, nil
}
