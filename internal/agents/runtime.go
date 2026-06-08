package agents

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/approval"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/audit"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/commands"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/messages"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/middlewares"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/prompts"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/rules"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools"
	"github.com/HappyLadySauce/Eino-Agent-CLI/pkg/config"
)

// AgentRuntime owns ADK agents, runners, tools, and orchestration settings.
// AgentRuntime 持有 ADK Agent、Runner、工具与编排设置。
type AgentRuntime struct {
	mu sync.RWMutex

	model            *openai.ChatModel
	handlers         []adk.ChatModelAgentMiddleware
	registry         *AgentRegistry
	runners          map[string]*adk.Runner
	mainRunners      map[commands.SessionMode]*adk.Runner
	sessionID        string
	workspaceRoot    string
	dataDir          string
	sandboxMode      security.SandboxMode
	approvalMode     security.ApprovalMode
	prompter         approval.Prompter
	auditSink        audit.Sink
	ruleSet          *rules.Set
	rateLimiter      *security.RateLimiter
	maxContextTokens int
}

// NewAgentRuntime creates the model, dynamic agent registry, main agents, and runners.
// NewAgentRuntime 创建模型、动态 Agent 注册表、主 Agent 与 Runner。
func NewAgentRuntime(ctx context.Context, cfg *config.Config, prompter approval.Prompter) (*AgentRuntime, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if cfg == nil || cfg.Model == nil {
		return nil, errors.New("agent config is missing model settings")
	}
	if cfg.Security == nil {
		return nil, errors.New("agent config is missing security settings")
	}

	modelConfig := &openai.ChatModelConfig{
		APIKey:  cfg.Model.AuthToken,
		BaseURL: cfg.Model.BaseURL,
		Model:   cfg.Model.Model,
	}
	if cfg.Model.MaxOutputTokens > 0 {
		modelConfig.MaxCompletionTokens = &cfg.Model.MaxOutputTokens
	}
	model, err := openai.NewChatModel(ctx, modelConfig)
	if err != nil {
		return nil, fmt.Errorf("create chat model: %w", err)
	}

	contextMiddleware, err := middlewares.NewContextMiddleware(middlewares.ContextMiddlewareConfig{
		ModelName:          cfg.Model.Model,
		TokenizerModel:     cfg.Model.TokenizerModel,
		MaxContextTokens:   cfg.Model.MaxContextTokens,
		MaxOutputTokens:    cfg.Model.MaxOutputTokens,
		MaxHistoryMessages: cfg.Model.MaxHistoryMessages,
	})
	if err != nil {
		return nil, fmt.Errorf("create context middleware: %w", err)
	}

	registry, err := NewAgentRegistry(nil)
	if err != nil {
		return nil, fmt.Errorf("create agent registry: %w", err)
	}
	sessionID, err := security.NewSessionID()
	if err != nil {
		return nil, err
	}
	workspaceRoot, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	workspaceRoot, err = filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("make workspace root absolute: %w", err)
	}
	dataDir, err := filepath.Abs(cfg.Security.DataDir)
	if err != nil {
		return nil, fmt.Errorf("make data directory absolute: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	ruleSet, err := rules.LoadFiles(
		filepath.Join(workspaceRoot, ".eino", "rules.star"),
		filepath.Join(dataDir, "rules.star"),
	)
	if err != nil {
		return nil, fmt.Errorf("load rules: %w", err)
	}
	auditPath := filepath.Join(dataDir, "audit", sessionID+".jsonl")
	auditSink := audit.MultiSink{audit.NewMemorySink(), audit.NewFileSink(auditPath)}

	runtime := &AgentRuntime{
		model:            model,
		handlers:         []adk.ChatModelAgentMiddleware{contextMiddleware},
		registry:         registry,
		runners:          make(map[string]*adk.Runner),
		mainRunners:      make(map[commands.SessionMode]*adk.Runner),
		sessionID:        sessionID,
		workspaceRoot:    workspaceRoot,
		dataDir:          dataDir,
		sandboxMode:      security.SandboxMode(cfg.Security.SandboxMode),
		approvalMode:     security.ApprovalMode(cfg.Security.ApprovalMode),
		prompter:         prompter,
		auditSink:        auditSink,
		ruleSet:          ruleSet,
		rateLimiter:      security.NewRateLimiter(),
		maxContextTokens: cfg.Model.MaxContextTokens,
	}

	for _, mode := range []commands.SessionMode{commands.SessionModeAgent, commands.SessionModePlan, commands.SessionModeAsk} {
		runner, err := runtime.newMainRunner(ctx, mode)
		if err != nil {
			return nil, fmt.Errorf("create main agent for mode %q: %w", mode, err)
		}
		runtime.mainRunners[mode] = runner
	}

	return runtime, nil
}

// Definitions returns all registered agent definitions.
// Definitions 返回所有已注册 Agent 定义。
func (r *AgentRuntime) Definitions() []AgentDefinition {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.registry.List()
}

// MaxContextTokens returns the configured model context window size.
// MaxContextTokens 返回配置的模型最大上下文 token 数。
func (r *AgentRuntime) MaxContextTokens() int {
	if r == nil {
		return 0
	}
	return r.maxContextTokens
}

// SessionID returns the audit correlation id for this runtime.
// SessionID 返回该运行时的审计关联 ID。
func (r *AgentRuntime) SessionID() string {
	if r == nil {
		return ""
	}
	return r.sessionID
}

// RunMain runs the main agent with shared conversation history in the selected mode.
// RunMain 在指定模式下使用共享对话历史运行主 Agent。
func (r *AgentRuntime) RunMain(ctx context.Context, mode commands.SessionMode, history []*schema.Message, writer io.Writer) (*messages.AssistantStreamResult, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if r == nil {
		return nil, errors.New("agent runtime is nil")
	}
	r.mu.RLock()
	runner := r.mainRunners[mode]
	r.mu.RUnlock()
	if runner == nil {
		return nil, fmt.Errorf("main agent runner for mode %q is not initialized", mode)
	}
	result, err := messages.ConsumeAssistantStream(runner.Run(ctx, history), writer)
	if err != nil {
		return nil, fmt.Errorf("consume assistant stream for mode %q: %w", mode, err)
	}
	return result, nil
}

// CreateAgent creates and registers a dynamic sub-agent.
// CreateAgent 创建并注册动态子 Agent。
func (r *AgentRuntime) CreateAgent(ctx context.Context, mode string, input tools.CreateAgentInput) (*tools.CreateAgentOutput, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if r == nil {
		return nil, errors.New("agent runtime is nil")
	}

	permission, err := validateCreateAgentPermission(commands.SessionMode(mode), input.PermissionMode)
	if err != nil {
		return nil, err
	}

	definition := AgentDefinition{
		Name:           normalizeAgentName(input.Name),
		Description:    strings.TrimSpace(input.Description),
		SystemPrompt:   strings.TrimSpace(input.Instruction),
		PermissionMode: permission,
		Dynamic:        true,
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.registry.Register(definition); err != nil {
		return nil, fmt.Errorf("register dynamic agent: %w", err)
	}
	if err := r.createRunnerLocked(ctx, definition); err != nil {
		return nil, fmt.Errorf("create dynamic agent runner: %w", err)
	}

	return &tools.CreateAgentOutput{
		Name:           definition.Name,
		Description:    definition.Description,
		PermissionMode: string(definition.PermissionMode),
		Created:        true,
	}, nil
}

// RunSubAgent runs a sub-agent with a fresh isolated context and returns only the final result.
// RunSubAgent 使用全新隔离上下文运行子 Agent，并仅返回最终结果。
func (r *AgentRuntime) RunSubAgent(ctx context.Context, mode string, input tools.RunSubAgentInput) (*tools.RunSubAgentOutput, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if r == nil {
		return nil, errors.New("agent runtime is nil")
	}
	task := strings.TrimSpace(input.Task)
	if task == "" {
		return nil, errors.New("sub-agent task is required")
	}
	sessionMode := commands.SessionMode(mode)
	if sessionMode == commands.SessionModeAsk {
		return nil, errors.New("ask mode cannot run sub-agents")
	}

	agentName := strings.TrimSpace(input.AgentName)
	if err := validateRunSubAgentRequest(sessionMode, agentName); err != nil {
		return nil, err
	}

	r.mu.RLock()
	definition, err := r.registry.Get(agentName)
	runner := r.runners[agentName]
	r.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	if runner == nil {
		return nil, fmt.Errorf("agent runner %q is not initialized", agentName)
	}
	if sessionMode == commands.SessionModePlan && !definition.PermissionMode.IsSubAgentSafeInPlan() {
		return nil, fmt.Errorf("plan mode cannot run agent %q with permission %q", agentName, definition.PermissionMode)
	}

	prompt := prompts.SubAgentPrompt(
		strings.TrimSpace(input.Task),
		strings.TrimSpace(input.ContextSummary),
		strings.TrimSpace(input.ExpectedOutput),
	)
	start := time.Now()
	result, err := messages.ConsumeAssistantStream(runner.Run(ctx, []*schema.Message{schema.UserMessage(prompt)}), io.Discard)
	if err != nil {
		return nil, fmt.Errorf("consume sub-agent %q result: %w", agentName, err)
	}
	return &tools.RunSubAgentOutput{
		AgentName:  agentName,
		Content:    result.Content,
		Created:    definition.Dynamic,
		Duration:   time.Since(start).String(),
		EventCount: result.EventCount,
		ChunkCount: result.ChunkCount,
	}, nil
}

// ListAgents returns the current built-in and dynamic agents.
// ListAgents 返回当前内置与动态 Agent。
func (r *AgentRuntime) ListAgents(_ context.Context, _ string, _ tools.ListAgentsInput) (*tools.ListAgentsOutput, error) {
	if r == nil {
		return nil, errors.New("agent runtime is nil")
	}
	definitions := r.Definitions()
	agents := make([]tools.AgentSummary, 0, len(definitions))
	for _, definition := range definitions {
		agents = append(agents, tools.AgentSummary{
			Name:           definition.Name,
			Description:    definition.Description,
			PermissionMode: string(definition.PermissionMode),
			Dynamic:        definition.Dynamic,
		})
	}
	return &tools.ListAgentsOutput{Agents: agents}, nil
}

func (r *AgentRuntime) createRunnerLocked(ctx context.Context, definition AgentDefinition) error {
	agent, err := newChatModelAgent(ctx, r.model, definition, nil, r.handlers)
	if err != nil {
		return err
	}
	r.runners[definition.Name] = adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})
	return nil
}

func (r *AgentRuntime) newMainRunner(ctx context.Context, mode commands.SessionMode) (*adk.Runner, error) {
	definition := AgentDefinition{
		Name:           string(mode) + "-main",
		Description:    "Main CLI agent for " + string(mode) + " mode.",
		SystemPrompt:   prompts.MainAgentInstruction(string(mode)),
		PermissionMode: PermissionModeDefault,
	}

	var tools []tool.BaseTool
	if mode == commands.SessionModeAgent || mode == commands.SessionModePlan {
		var err error
		tools, err = r.agentToolsForMode(mode)
		if err != nil {
			return nil, err
		}
	}

	agent, err := newChatModelAgent(ctx, r.model, definition, tools, r.handlers)
	if err != nil {
		return nil, err
	}
	return adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	}), nil
}

func (r *AgentRuntime) agentToolsForMode(mode commands.SessionMode) ([]tool.BaseTool, error) {
	return tools.NewSecureAgentTools(string(mode), r, tools.SecureToolOptions{
		Context:     r.securityContextForMode(mode),
		Prompter:    r.prompter,
		AuditSink:   r.auditSink,
		RuleSet:     r.ruleSet,
		RateLimiter: r.rateLimiter,
	})
}

func (r *AgentRuntime) securityContextForMode(mode commands.SessionMode) security.Context {
	secMode := security.SessionMode(mode)
	sandbox := r.sandboxMode
	approvalMode := r.approvalMode
	switch secMode {
	case security.SessionModeAsk:
		sandbox = security.SandboxModeReadOnly
		approvalMode = security.ApprovalModeNever
	case security.SessionModePlan:
		if sandbox == security.SandboxModeWorkspaceWrite {
			sandbox = security.SandboxModeReadOnly
		}
		if approvalMode == security.ApprovalModeAuto {
			approvalMode = security.ApprovalModeInteractive
		}
	}
	return security.Context{
		SessionID:     r.sessionID,
		WorkspaceRoot: r.workspaceRoot,
		DataDir:       r.dataDir,
		SessionMode:   secMode,
		SandboxMode:   sandbox,
		ApprovalMode:  approvalMode,
	}
}

func newChatModelAgent(ctx context.Context, model *openai.ChatModel, definition AgentDefinition, tools []tool.BaseTool, handlers []adk.ChatModelAgentMiddleware) (*adk.ChatModelAgent, error) {
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
		Handlers:    handlers,
	}
	if len(tools) > 0 {
		config.ToolsConfig = adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
			EmitInternalEvents: false,
		}
	}
	agent, err := adk.NewChatModelAgent(ctx, config)
	if err != nil {
		return nil, err
	}
	return agent, nil
}

func isKnownPermissionMode(mode PermissionMode) bool {
	switch mode {
	case PermissionModeDefault, PermissionModeReadonly, PermissionModePlan:
		return true
	default:
		return false
	}
}

func validateCreateAgentPermission(sessionMode commands.SessionMode, rawPermission string) (PermissionMode, error) {
	permissionText := strings.TrimSpace(rawPermission)
	if permissionText == "" {
		return "", errors.New("permission_mode is required")
	}
	permission := PermissionMode(permissionText)
	if sessionMode == commands.SessionModeAsk {
		return "", errors.New("ask mode cannot create sub-agents")
	}
	if !isKnownPermissionMode(permission) {
		return "", fmt.Errorf("unsupported permission mode %q", permission)
	}
	if sessionMode == commands.SessionModePlan && !permission.IsSubAgentSafeInPlan() {
		return "", fmt.Errorf("plan mode cannot create agent with permission %q", permission)
	}
	return permission, nil
}

func validateRunSubAgentRequest(sessionMode commands.SessionMode, agentName string) error {
	if sessionMode == commands.SessionModeAsk {
		return errors.New("ask mode cannot run sub-agents")
	}
	if strings.TrimSpace(agentName) == "" {
		return errors.New("agent_name is required")
	}
	return nil
}

func normalizeAgentName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.Join(strings.Fields(name), "-")
	return name
}
