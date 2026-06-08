package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/approval"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/audit"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/rules"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
)

// SecureToolOptions contains shared dependencies for secured tools.
// SecureToolOptions 包含安全工具的共享依赖。
type SecureToolOptions struct {
	Context     security.Context
	Prompter    approval.Prompter
	AuditSink   audit.Sink
	RuleSet     *rules.Set
	RateLimiter *security.RateLimiter
}

// Runtime contains dependencies available to every tool definition.
// Runtime 包含每个工具定义可用的依赖。
type Runtime struct {
	Mode    string
	Service any
	Options SecureToolOptions
}

// Definition is the common contract implemented by every tool.
// Definition 是每个工具实现的通用契约。
type Definition interface {
	Name() string
	Enabled(Runtime) bool
	Build(Runtime) (einotool.BaseTool, error)
}

// GenericDefinition adapts typed handlers into secured Eino tools.
// GenericDefinition 将类型化 handler 适配为安全 Eino 工具。
type GenericDefinition[I any, O any] struct {
	Descriptor  security.ToolDescriptor
	Description string
	Enable      func(Runtime) bool
	Request     func(Runtime, I) security.OperationRequest
	Tokens      func(Runtime, I) []string
	Handler     func(context.Context, Runtime, I) (*O, error)
}

type secureHandler[I any, O any] func(context.Context, I) (*O, error)
type requestBuilder[I any] func(I) security.OperationRequest
type tokenBuilder[I any] func(I) []string

type secureToolFactory struct {
	opts SecureToolOptions
}

// Name returns the Eino tool name.
// Name 返回 Eino 工具名称。
func (d GenericDefinition[I, O]) Name() string {
	return d.Descriptor.Name
}

// Enabled reports whether the tool should be exposed for this runtime.
// Enabled 判断工具是否应在当前运行时暴露。
func (d GenericDefinition[I, O]) Enabled(runtime Runtime) bool {
	if d.Enable == nil {
		return true
	}
	return d.Enable(runtime)
}

// Build creates one secured Eino tool.
// Build 创建一个安全 Eino 工具。
func (d GenericDefinition[I, O]) Build(runtime Runtime) (einotool.BaseTool, error) {
	if d.Request == nil {
		return nil, fmt.Errorf("tool %q request builder is nil", d.Name())
	}
	if d.Handler == nil {
		return nil, fmt.Errorf("tool %q handler is nil", d.Name())
	}
	tokens := d.Tokens
	if tokens == nil {
		tokens = func(Runtime, I) []string { return nil }
	}
	factory := secureToolFactory{opts: runtime.Options.WithDefaults()}
	return secureInfer[I, O](
		factory,
		d.Descriptor,
		d.Description,
		func(input I) security.OperationRequest { return d.Request(runtime, input) },
		func(input I) []string { return tokens(runtime, input) },
		func(ctx context.Context, input I) (*O, error) { return d.Handler(ctx, runtime, input) },
	)
}

// WithDefaults fills optional dependencies with safe defaults.
// WithDefaults 使用安全默认值填充可选依赖。
func (o SecureToolOptions) WithDefaults() SecureToolOptions {
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

// BuildDefinitions creates all enabled tools from definitions.
// BuildDefinitions 从工具定义创建所有启用工具。
func BuildDefinitions(runtime Runtime, definitions []Definition) ([]einotool.BaseTool, error) {
	definitions = append([]Definition(nil), definitions...)
	sort.SliceStable(definitions, func(i, j int) bool {
		return definitions[i].Name() < definitions[j].Name()
	})
	seen := map[string]bool{}
	out := make([]einotool.BaseTool, 0, len(definitions))
	for _, definition := range definitions {
		if seen[definition.Name()] {
			return nil, fmt.Errorf("duplicate tool definition %q", definition.Name())
		}
		seen[definition.Name()] = true
		if !definition.Enabled(runtime) {
			continue
		}
		tool, err := definition.Build(runtime)
		if err != nil {
			return nil, err
		}
		out = append(out, tool)
	}
	return out, nil
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

// ToSecuritySessionMode converts a CLI mode string to a security session mode.
// ToSecuritySessionMode 将 CLI 模式字符串转换为安全会话模式。
func ToSecuritySessionMode(mode string) security.SessionMode {
	switch mode {
	case string(security.SessionModeAsk):
		return security.SessionModeAsk
	case string(security.SessionModePlan):
		return security.SessionModePlan
	default:
		return security.SessionModeAgent
	}
}

// DefaultDataDir returns the default Eino data directory.
// DefaultDataDir 返回默认 Eino 数据目录。
func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", ".eino")
	}
	return filepath.Join(home, "eino")
}

// ResolvePath resolves a tool path inside the active workspace.
// ResolvePath 在当前工作区内解析工具路径。
func ResolvePath(runtime Runtime, path string, operation security.OperationKind) (string, error) {
	resolved, err := security.ResolveWorkspacePath(runtime.Options.Context.WorkspaceRoot, path, operation)
	if err != nil {
		return "", err
	}
	return resolved.Absolute, nil
}

// ResolvePathRequest resolves a path for policy metadata.
// ResolvePathRequest 为策略元数据解析路径。
func ResolvePathRequest(runtime Runtime, path string, operation security.OperationKind) (string, security.OperationRisk) {
	resolved, err := security.ResolveWorkspacePath(runtime.Options.Context.WorkspaceRoot, path, operation)
	if err != nil {
		return path, security.OperationRiskUnknown
	}
	return resolved.Absolute, security.OperationRiskLow
}

// OperationForFileWrite returns the operation kind for a file write input.
// OperationForFileWrite 返回文件写入输入对应的操作类型。
func OperationForFileWrite(bool) security.OperationKind {
	return security.OperationWrite
}

// SafeName normalizes a user-provided data entry name.
// SafeName 规范化用户提供的数据项名称。
func SafeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	return filepath.Clean(name)
}
