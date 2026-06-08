package security

import (
	"errors"
	"fmt"
	"strings"
)

// ValidateToolDescriptor checks whether a descriptor is safe to register.
// ValidateToolDescriptor 校验工具描述符是否可安全注册。
func ValidateToolDescriptor(descriptor ToolDescriptor) error {
	if strings.TrimSpace(descriptor.Name) == "" {
		return errors.New("tool name is required")
	}
	switch descriptor.Provider {
	case ToolProviderBuiltin, ToolProviderMCP, ToolProviderPlugin:
	default:
		return fmt.Errorf("unsupported tool provider %q", descriptor.Provider)
	}
	if descriptor.Kind == "" {
		return errors.New("tool kind is required")
	}
	if descriptor.Risk == "" {
		return errors.New("operation risk is required")
	}
	if descriptor.Kind == ToolKindNetwork || descriptor.Kind == ToolKindExternalState {
		if len(descriptor.Resources) == 0 {
			return errors.New("external tools must declare resources")
		}
	}
	return nil
}

// BuiltInDecision returns the deterministic built-in default policy decision.
// BuiltInDecision 返回确定性的内置默认策略决策。
func BuiltInDecision(ctx Context, req OperationRequest) PolicyDecision {
	if err := ctx.Validate(); err != nil {
		return PolicyDecision{Decision: DecisionDeny, Reason: err.Error()}
	}
	if err := ValidateToolDescriptor(req.Tool); err != nil {
		return PolicyDecision{Decision: DecisionDeny, Reason: err.Error()}
	}
	risk := req.Risk
	if risk == "" {
		risk = req.Tool.Risk
	}
	if risk == OperationRiskUnknown || req.Unknown {
		return PolicyDecision{Decision: DecisionDeny, Reason: "unknown operation risk is denied by default"}
	}
	if ctx.SandboxMode == SandboxModeDangerFullAccess {
		return dangerFullAccessDecision(req, risk)
	}
	if ctx.SessionMode == SessionModeAsk {
		return askModeDecision(req, risk)
	}
	if ctx.SessionMode == SessionModePlan {
		return planModeDecision(req, risk)
	}
	return agentModeDecision(ctx, req, risk)
}

// ApplyApprovalMode reduces a policy decision according to the approval mode.
// ApplyApprovalMode 根据审批模式折算策略决策。
func ApplyApprovalMode(mode ApprovalMode, decision PolicyDecision) ToolResult[struct{}] {
	switch decision.Decision {
	case DecisionAllow:
		return ToolResult[struct{}]{OK: true, Status: ResultStatusOK}
	case DecisionSuggest:
		return ToolResult[struct{}]{OK: false, Status: ResultStatusSuggested, Reason: decision.Reason}
	case DecisionDeny:
		return ToolResult[struct{}]{OK: false, Status: ResultStatusDenied, Reason: decision.Reason}
	case DecisionAutoEligible:
		if mode == ApprovalModeAuto {
			return ToolResult[struct{}]{OK: true, Status: ResultStatusOK}
		}
		if mode == ApprovalModeNever {
			return ToolResult[struct{}]{OK: false, Status: ResultStatusDenied, Reason: decision.Reason}
		}
		return ToolResult[struct{}]{OK: false, Status: ResultStatusApprovalRequired, Reason: decision.Reason}
	case DecisionAsk:
		if mode == ApprovalModeNever {
			return ToolResult[struct{}]{OK: false, Status: ResultStatusDenied, Reason: decision.Reason}
		}
		return ToolResult[struct{}]{OK: false, Status: ResultStatusApprovalRequired, Reason: decision.Reason}
	default:
		return ToolResult[struct{}]{OK: false, Status: ResultStatusDenied, Reason: "unknown policy decision"}
	}
}

func dangerFullAccessDecision(req OperationRequest, risk OperationRisk) PolicyDecision {
	if risk == OperationRiskLow && req.Tool.AutoApprove {
		return PolicyDecision{Decision: DecisionAutoEligible, Reason: "low-risk operation is auto eligible"}
	}
	return PolicyDecision{Decision: DecisionAsk, Reason: "danger-full-access operation requires approval by default"}
}

func askModeDecision(req OperationRequest, risk OperationRisk) PolicyDecision {
	if isFileReadLike(req) && risk == OperationRiskLow {
		return PolicyDecision{Decision: DecisionAllow, Reason: "ask mode allows low-risk file read/list"}
	}
	if isFileReadLike(req) && risk == OperationRiskMedium {
		return PolicyDecision{Decision: DecisionAsk, Reason: "sensitive file read requires approval"}
	}
	return PolicyDecision{Decision: DecisionDeny, Reason: "ask mode denies commands, writes, deletes, agents, and external tools"}
}

func planModeDecision(req OperationRequest, risk OperationRisk) PolicyDecision {
	if isFileReadLike(req) && risk == OperationRiskLow {
		return PolicyDecision{Decision: DecisionAllow, Reason: "plan mode allows low-risk file read/list"}
	}
	if isFileReadLike(req) && risk == OperationRiskMedium {
		return PolicyDecision{Decision: DecisionAsk, Reason: "sensitive file read requires approval"}
	}
	if req.Tool.Kind == ToolKindCommand && req.Operation == OperationExec && risk == OperationRiskLow {
		return PolicyDecision{Decision: DecisionAllow, Reason: "plan mode allows known local read-only commands"}
	}
	if req.Tool.Kind == ToolKindFileWrite || req.Tool.Kind == ToolKindFileDelete || req.External || req.Network {
		if risk == OperationRiskHigh || risk == OperationRiskDestructive {
			return PolicyDecision{Decision: DecisionDeny, Reason: "high-risk mutation is denied in plan mode"}
		}
		return PolicyDecision{Decision: DecisionSuggest, Reason: "plan mode returns suggestions for mutating operations"}
	}
	if req.Tool.Kind == ToolKindAgent {
		if risk == OperationRiskLow || risk == OperationRiskMedium {
			return PolicyDecision{Decision: DecisionAllow, Reason: "plan mode allows bounded agent orchestration"}
		}
		return PolicyDecision{Decision: DecisionDeny, Reason: "high-risk agent operation is denied in plan mode"}
	}
	if req.Tool.Kind == ToolKindMemory {
		if req.Operation == OperationRead && risk == OperationRiskLow {
			return PolicyDecision{Decision: DecisionAllow, Reason: "plan mode allows low-risk memory read"}
		}
		return PolicyDecision{Decision: DecisionSuggest, Reason: "plan mode returns suggestions for memory writes"}
	}
	return PolicyDecision{Decision: DecisionDeny, Reason: "operation is not allowed in plan mode"}
}

func agentModeDecision(ctx Context, req OperationRequest, risk OperationRisk) PolicyDecision {
	if ctx.SandboxMode != SandboxModeWorkspaceWrite {
		return PolicyDecision{Decision: DecisionDeny, Reason: "agent mode requires workspace-write sandbox unless danger-full-access is enabled"}
	}
	if isFileReadLike(req) && risk == OperationRiskLow {
		return PolicyDecision{Decision: DecisionAllow, Reason: "agent mode allows low-risk file read/list"}
	}
	switch req.Tool.Kind {
	case ToolKindFileWrite:
		if risk == OperationRiskLow || risk == OperationRiskMedium {
			return PolicyDecision{Decision: DecisionAutoEligible, Reason: "workspace file create/update is auto eligible"}
		}
		return PolicyDecision{Decision: DecisionAsk, Reason: "high-risk file write requires approval"}
	case ToolKindFileDelete:
		return PolicyDecision{Decision: DecisionAsk, Reason: "file delete requires approval"}
	case ToolKindCommand:
		if risk == OperationRiskLow && req.Tool.AutoApprove {
			return PolicyDecision{Decision: DecisionAutoEligible, Reason: "known read-like command is auto eligible"}
		}
		return PolicyDecision{Decision: DecisionAsk, Reason: "command execution requires approval by default"}
	case ToolKindNetwork, ToolKindExternalState:
		return PolicyDecision{Decision: DecisionAsk, Reason: "external operation requires approval by default"}
	case ToolKindAgent:
		if risk == OperationRiskLow {
			return PolicyDecision{Decision: DecisionAllow, Reason: "low-risk agent tool is allowed"}
		}
		return PolicyDecision{Decision: DecisionAsk, Reason: "agent state mutation requires approval"}
	case ToolKindMemory:
		if req.Operation == OperationRead && risk == OperationRiskLow {
			return PolicyDecision{Decision: DecisionAllow, Reason: "low-risk memory read is allowed"}
		}
		return PolicyDecision{Decision: DecisionAsk, Reason: "memory mutation requires approval"}
	default:
		return PolicyDecision{Decision: DecisionDeny, Reason: "unsupported tool kind is denied"}
	}
}

func isFileReadLike(req OperationRequest) bool {
	return req.Tool.Kind == ToolKindFileRead && (req.Operation == OperationRead || req.Operation == OperationList)
}
