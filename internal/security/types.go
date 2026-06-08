package security

import "time"

// SessionMode describes the user's current interaction intent.
// SessionMode 描述用户当前的交互意图。
type SessionMode string

const (
	SessionModeAsk   SessionMode = "ask"
	SessionModePlan  SessionMode = "plan"
	SessionModeAgent SessionMode = "agent"
)

// SandboxMode describes the maximum capability ceiling for a session.
// SandboxMode 描述会话的最高能力边界。
type SandboxMode string

const (
	SandboxModeReadOnly         SandboxMode = "read-only"
	SandboxModeWorkspaceWrite   SandboxMode = "workspace-write"
	SandboxModeDangerFullAccess SandboxMode = "danger-full-access"
)

// ApprovalMode controls how policy-sensitive operations are handled.
// ApprovalMode 控制敏感策略操作的审批方式。
type ApprovalMode string

const (
	ApprovalModeInteractive ApprovalMode = "interactive"
	ApprovalModeAuto        ApprovalMode = "auto"
	ApprovalModeNever       ApprovalMode = "never"
)

// ToolProvider identifies where a tool implementation comes from.
// ToolProvider 标识工具实现的来源。
type ToolProvider string

const (
	ToolProviderBuiltin ToolProvider = "builtin"
	ToolProviderMCP     ToolProvider = "mcp"
	ToolProviderPlugin  ToolProvider = "plugin"
)

// ToolKind describes the broad behavior surface of a tool.
// ToolKind 描述工具的大类行为面。
type ToolKind string

const (
	ToolKindFileRead      ToolKind = "file_read"
	ToolKindFileWrite     ToolKind = "file_write"
	ToolKindFileDelete    ToolKind = "file_delete"
	ToolKindCommand       ToolKind = "command"
	ToolKindAgent         ToolKind = "agent"
	ToolKindNetwork       ToolKind = "network"
	ToolKindExternalState ToolKind = "external_state"
	ToolKindMemory        ToolKind = "memory"
)

// OperationKind describes the specific action a rule or request applies to.
// OperationKind 描述规则或请求适用的具体操作。
type OperationKind string

const (
	OperationRead          OperationKind = "read"
	OperationList          OperationKind = "list"
	OperationWrite         OperationKind = "write"
	OperationDelete        OperationKind = "delete"
	OperationExec          OperationKind = "exec"
	OperationNetwork       OperationKind = "network"
	OperationUpload        OperationKind = "upload"
	OperationMemoryWrite   OperationKind = "memory_write"
	OperationExternalState OperationKind = "external_state"
)

// OperationRisk is the normalized risk level used by built-in policy.
// OperationRisk 是内置策略使用的标准化风险级别。
type OperationRisk string

const (
	OperationRiskLow         OperationRisk = "low"
	OperationRiskMedium      OperationRisk = "medium"
	OperationRiskHigh        OperationRisk = "high"
	OperationRiskDestructive OperationRisk = "destructive"
	OperationRiskUnknown     OperationRisk = "unknown"
)

// Decision is a policy decision before approval-mode reduction.
// Decision 是审批模式折算前的策略决策。
type Decision string

const (
	DecisionAllow        Decision = "allow"
	DecisionAsk          Decision = "ask"
	DecisionAutoEligible Decision = "auto-eligible"
	DecisionSuggest      Decision = "suggest"
	DecisionDeny         Decision = "deny"
)

// ResultStatus is returned to the model through structured tool results.
// ResultStatus 通过结构化工具结果返回给模型。
type ResultStatus string

const (
	ResultStatusOK               ResultStatus = "ok"
	ResultStatusDenied           ResultStatus = "denied"
	ResultStatusApprovalRequired ResultStatus = "approval_required"
	ResultStatusRejected         ResultStatus = "rejected"
	ResultStatusFailed           ResultStatus = "failed"
	ResultStatusTimeout          ResultStatus = "timeout"
	ResultStatusTruncated        ResultStatus = "truncated"
	ResultStatusSuggested        ResultStatus = "suggested"
	ResultStatusRateLimited      ResultStatus = "rate_limited"
)

// ResourceDescriptor declares local or external resources used by a tool.
// ResourceDescriptor 声明工具使用的本地或外部资源。
type ResourceDescriptor struct {
	Kind     string   `json:"kind"`
	Targets  []string `json:"targets,omitempty"`
	Unknown  bool     `json:"unknown,omitempty"`
	ReadOnly bool     `json:"read_only,omitempty"`
}

// RateLimitDescriptor declares default rate limits for a tool.
// RateLimitDescriptor 声明工具的默认速率限制。
type RateLimitDescriptor struct {
	MaxCallsPerSession int           `json:"max_calls_per_session,omitempty"`
	MaxCallsPerMinute  int           `json:"max_calls_per_minute,omitempty"`
	MaxRuntime         time.Duration `json:"max_runtime,omitempty"`
	MaxOutputBytes     int64         `json:"max_output_bytes,omitempty"`
}

// ToolDescriptor is mandatory metadata for every exposed tool.
// ToolDescriptor 是每个暴露工具的强制元数据。
type ToolDescriptor struct {
	Name           string               `json:"name"`
	Provider       ToolProvider         `json:"provider"`
	Kind           ToolKind             `json:"kind"`
	Risk           OperationRisk        `json:"risk"`
	AutoApprove    bool                 `json:"auto_approve"`
	Resources      []ResourceDescriptor `json:"resources,omitempty"`
	SupportsDryRun bool                 `json:"supports_dry_run,omitempty"`
	RateLimit      *RateLimitDescriptor `json:"rate_limit,omitempty"`
}

// OperationRequest is the normalized input to policy evaluation.
// OperationRequest 是策略评估的标准化输入。
type OperationRequest struct {
	SessionID   string
	Tool        ToolDescriptor
	Operation   OperationKind
	TargetPath  string
	Command     string
	CWD         string
	Risk        OperationRisk
	External    bool
	Unknown     bool
	Network     bool
	Description string
}

// PolicyDecision contains the decision and reason from policy evaluation.
// PolicyDecision 包含策略评估给出的决策和原因。
type PolicyDecision struct {
	Decision Decision
	Reason   string
}

// ToolResult is the generic envelope returned to the model.
// ToolResult 是返回给模型的通用工具结果信封。
type ToolResult[T any] struct {
	OK         bool         `json:"ok"`
	Status     ResultStatus `json:"status"`
	Reason     string       `json:"reason,omitempty"`
	Suggestion string       `json:"suggestion,omitempty"`
	Data       T            `json:"data,omitempty"`
	Truncated  bool         `json:"truncated,omitempty"`
	AuditID    string       `json:"audit_id,omitempty"`
}
