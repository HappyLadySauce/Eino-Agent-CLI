package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/approval"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/audit"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/rules"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
)

// AgentToolService is the runtime surface used by agent orchestration tools.
// AgentToolService 是 Agent 编排工具依赖的运行时服务面。
type AgentToolService interface {
	ListAgents(ctx context.Context, mode string, input ListAgentsInput) (*ListAgentsOutput, error)
	CreateAgent(ctx context.Context, mode string, input CreateAgentInput) (*CreateAgentOutput, error)
	RunSubAgent(ctx context.Context, mode string, input RunSubAgentInput) (*RunSubAgentOutput, error)
}

// SecureToolOptions contains shared dependencies for secured tools.
// SecureToolOptions 包含安全工具的共享依赖。
type SecureToolOptions struct {
	Context     security.Context
	Prompter    approval.Prompter
	AuditSink   audit.Sink
	RuleSet     *rules.Set
	RateLimiter *security.RateLimiter
}

type secureToolFactory struct {
	mode    string
	service AgentToolService
	opts    SecureToolOptions
}

type secureHandler[I any, O any] func(context.Context, I) (*O, error)
type requestBuilder[I any] func(I) security.OperationRequest
type tokenBuilder[I any] func(I) []string

// NewAgentTools creates all main-agent tools for the given session mode.
// NewAgentTools 为指定会话模式创建主 Agent 工具集合。
func NewAgentTools(mode string, service AgentToolService) ([]einotool.BaseTool, error) {
	workspace, _ := os.Getwd()
	dataDir := defaultDataDir()
	secCtx, err := security.DefaultContextForSession("test-session", workspace, dataDir, toSecuritySessionMode(mode))
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
	factory := secureToolFactory{mode: mode, service: service, opts: opts.withDefaults()}
	var out []einotool.BaseTool
	add := func(tool einotool.BaseTool, err error) error {
		if err != nil {
			return err
		}
		out = append(out, tool)
		return nil
	}

	if err := add(factory.listAgentsTool()); err != nil {
		return nil, err
	}
	if mode != string(security.SessionModeAsk) {
		if err := add(factory.createAgentTool()); err != nil {
			return nil, err
		}
		if err := add(factory.runSubAgentTool()); err != nil {
			return nil, err
		}
	}
	for _, build := range []func() (einotool.BaseTool, error){
		factory.readFileTool,
		factory.listDirTool,
		factory.createFileTool,
		factory.patchFileTool,
		factory.replaceFileTool,
		factory.deleteFileTool,
		factory.runCommandTool,
		factory.listMemoryTool,
		factory.readMemoryTool,
		factory.writeMemoryTool,
		factory.sessionInfoTool,
	} {
		if err := add(build()); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (o SecureToolOptions) withDefaults() SecureToolOptions {
	if o.AuditSink == nil {
		o.AuditSink = audit.NewMemorySink()
	}
	if o.RuleSet == nil {
		o.RuleSet = rules.NewSet()
	}
	if o.RateLimiter == nil {
		o.RateLimiter = security.NewRateLimiter()
	}
	if o.Prompter == nil {
		o.Prompter = &approval.FakePrompter{Decision: approval.DecisionDeny}
	}
	return o
}

func secureInfer[I any, O any](
	f secureToolFactory,
	descriptor security.ToolDescriptor,
	description string,
	build requestBuilder[I],
	tokens tokenBuilder[I],
	handler secureHandler[I, O],
) (einotool.BaseTool, error) {
	registry := security.NewRegistry()
	if err := registry.Register(descriptor); err != nil {
		return nil, err
	}
	if _, err := registry.MustExpose(descriptor.Provider, descriptor.Name); err != nil {
		return nil, err
	}
	return toolutils.InferTool[I, *security.ToolResult[*O]](
		descriptor.Name,
		description,
		func(ctx context.Context, input I) (*security.ToolResult[*O], error) {
			return invokeSecureTool(ctx, f, descriptor, build(input), tokens(input), input, handler)
		},
	)
}

func invokeSecureTool[I any, O any](
	ctx context.Context,
	f secureToolFactory,
	descriptor security.ToolDescriptor,
	req security.OperationRequest,
	tokens []string,
	input I,
	handler secureHandler[I, O],
) (*security.ToolResult[*O], error) {
	start := time.Now()
	req.SessionID = f.opts.Context.SessionID
	req.Tool = descriptor
	if req.Risk == "" {
		req.Risk = descriptor.Risk
	}
	auditID := audit.NewID()
	argumentSummary := summarizeInput(input)

	if descriptor.RateLimit != nil {
		if limited := f.opts.RateLimiter.CheckAndRecord(f.opts.Context.SessionID, descriptor.Name, *descriptor.RateLimit); limited != nil {
			limited.AuditID = auditID
			_ = f.appendAudit(ctx, auditID, descriptor, req, argumentSummary, security.DecisionDeny, "rate limited", "", time.Since(start), limited.Status)
			return ptrResult[O](limited), nil
		}
	}

	if hardDenied, reason := hardDeny(req); hardDenied {
		result := &security.ToolResult[*O]{OK: false, Status: security.ResultStatusDenied, Reason: reason, AuditID: auditID}
		_ = f.appendAudit(ctx, auditID, descriptor, req, argumentSummary, security.DecisionDeny, reason, "", time.Since(start), result.Status)
		f.opts.RateLimiter.RecordDenied(f.opts.Context.SessionID, descriptor.Name)
		return result, nil
	}

	decision, matched := f.opts.RuleSet.Evaluate(f.opts.Context, req, tokens)
	if !matched {
		decision = security.BuiltInDecision(f.opts.Context, req)
	}
	reduced := security.ApplyApprovalMode(f.opts.Context.ApprovalMode, decision)
	reduced.AuditID = auditID

	if reduced.Status == security.ResultStatusApprovalRequired {
		userDecision, err := f.opts.Prompter.Prompt(ctx, approval.Request{
			ToolName: descriptor.Name,
			CWD:      req.CWD,
			Input:    argumentSummary,
			Risk:     req.Risk,
			Reason:   decision.Reason,
		})
		if err != nil {
			return nil, err
		}
		if userDecision != approval.DecisionApproveOnce {
			result := &security.ToolResult[*O]{OK: false, Status: security.ResultStatusRejected, Reason: "operation rejected by user", AuditID: auditID}
			_ = f.appendAudit(ctx, auditID, descriptor, req, argumentSummary, decision.Decision, decision.Reason, string(userDecision), time.Since(start), result.Status)
			f.opts.RateLimiter.RecordDenied(f.opts.Context.SessionID, descriptor.Name)
			return result, nil
		}
	}
	if reduced.Status == security.ResultStatusDenied || reduced.Status == security.ResultStatusSuggested {
		result := &security.ToolResult[*O]{OK: false, Status: reduced.Status, Reason: reduced.Reason, Suggestion: reduced.Suggestion, AuditID: auditID}
		_ = f.appendAudit(ctx, auditID, descriptor, req, argumentSummary, decision.Decision, decision.Reason, "", time.Since(start), result.Status)
		if reduced.Status == security.ResultStatusDenied {
			f.opts.RateLimiter.RecordDenied(f.opts.Context.SessionID, descriptor.Name)
		}
		return result, nil
	}

	data, err := handler(ctx, input)
	if err != nil {
		result := &security.ToolResult[*O]{OK: false, Status: security.ResultStatusFailed, Reason: err.Error(), AuditID: auditID}
		_ = f.appendAudit(ctx, auditID, descriptor, req, argumentSummary, decision.Decision, decision.Reason, "", time.Since(start), result.Status)
		return result, nil
	}
	result := &security.ToolResult[*O]{OK: true, Status: security.ResultStatusOK, Data: data, AuditID: auditID}
	_ = f.appendAudit(ctx, auditID, descriptor, req, argumentSummary, decision.Decision, decision.Reason, "", time.Since(start), result.Status)
	return result, nil
}

func (f secureToolFactory) appendAudit(ctx context.Context, id string, descriptor security.ToolDescriptor, req security.OperationRequest, args string, decision security.Decision, reason string, userDecision string, duration time.Duration, status security.ResultStatus) error {
	return f.opts.AuditSink.Append(ctx, audit.Record{
		ID:           id,
		Timestamp:    time.Now(),
		SessionID:    f.opts.Context.SessionID,
		SessionMode:  f.opts.Context.SessionMode,
		SandboxMode:  f.opts.Context.SandboxMode,
		ApprovalMode: f.opts.Context.ApprovalMode,
		ToolName:     descriptor.Name,
		Provider:     descriptor.Provider,
		Operation:    req.Operation,
		CWD:          req.CWD,
		TargetPath:   req.TargetPath,
		Arguments:    args,
		Risk:         req.Risk,
		Decision:     decision,
		Reason:       reason,
		UserDecision: userDecision,
		Duration:     duration,
		ResultStatus: status,
	})
}

func hardDeny(req security.OperationRequest) (bool, string) {
	if req.TargetPath != "" && security.IsSensitivePath(req.TargetPath) {
		switch req.Operation {
		case security.OperationRead, security.OperationUpload, security.OperationDelete, security.OperationWrite:
			return true, "sensitive path is hard-denied"
		}
	}
	if req.Operation == security.OperationDelete {
		if err := security.ValidateDestructiveTarget(req.TargetPath); err != nil {
			return true, err.Error()
		}
	}
	return false, ""
}

func summarizeInput(input any) string {
	data, err := json.Marshal(input)
	if err != nil {
		return "<unserializable>"
	}
	return audit.Summarize(string(data), 512)
}

func ptrResult[O any](in *security.ToolResult[struct{}]) *security.ToolResult[*O] {
	return &security.ToolResult[*O]{
		OK:         in.OK,
		Status:     in.Status,
		Reason:     in.Reason,
		Suggestion: in.Suggestion,
		Truncated:  in.Truncated,
		AuditID:    in.AuditID,
	}
}

func toSecuritySessionMode(mode string) security.SessionMode {
	switch mode {
	case string(security.SessionModeAsk):
		return security.SessionModeAsk
	case string(security.SessionModePlan):
		return security.SessionModePlan
	default:
		return security.SessionModeAgent
	}
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", ".eino")
	}
	return filepath.Join(home, "eino")
}

func operationForFileWrite(dryRun bool) security.OperationKind {
	if dryRun {
		return security.OperationWrite
	}
	return security.OperationWrite
}

func safeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	return filepath.Clean(name)
}
