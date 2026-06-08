package security

import (
	"errors"
	"fmt"
	"strings"
)

// Context is the immutable security context shared by one session.
// Context 是单个会话共享的不可变安全上下文。
type Context struct {
	SessionID     string			// 会话ID
	WorkspaceRoot string			// 工作区根目录
	DataDir       string			// 数据目录
	SessionMode   SessionMode		// 会话模式
	SandboxMode   SandboxMode		// 沙箱模式
	ApprovalMode  ApprovalMode		// 审批模式
}

// Validate checks that the context is complete and internally consistent.
// Validate 校验上下文是否完整且内部一致。
func (c Context) Validate() error {
	if strings.TrimSpace(c.SessionID) == "" {
		return errors.New("session id is required")
	}
	if strings.TrimSpace(c.WorkspaceRoot) == "" {
		return errors.New("workspace root is required")
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return errors.New("data directory is required")
	}
	if !IsKnownSessionMode(c.SessionMode) {
		return fmt.Errorf("unsupported session mode %q", c.SessionMode)
	}
	if !IsKnownSandboxMode(c.SandboxMode) {
		return fmt.Errorf("unsupported sandbox mode %q", c.SandboxMode)
	}
	if !IsKnownApprovalMode(c.ApprovalMode) {
		return fmt.Errorf("unsupported approval mode %q", c.ApprovalMode)
	}
	switch c.SessionMode {
	case SessionModeAsk:
		if c.SandboxMode != SandboxModeReadOnly {
			return fmt.Errorf("ask mode requires read-only sandbox, got %q", c.SandboxMode)
		}
		if c.ApprovalMode != ApprovalModeNever {
			return fmt.Errorf("ask mode requires never approval, got %q", c.ApprovalMode)
		}
	case SessionModePlan:
		if c.SandboxMode == SandboxModeWorkspaceWrite {
			return errors.New("plan mode cannot use workspace-write sandbox by default")
		}
		if c.ApprovalMode == ApprovalModeAuto {
			return errors.New("plan mode cannot use auto approval by default")
		}
	}
	return nil
}

// IsKnownSessionMode reports whether the session mode is supported.
// IsKnownSessionMode 判断会话模式是否受支持。
func IsKnownSessionMode(mode SessionMode) bool {
	switch mode {
	case SessionModeAsk, SessionModePlan, SessionModeAgent:
		return true
	default:
		return false
	}
}

// IsKnownSandboxMode reports whether the sandbox mode is supported.
// IsKnownSandboxMode 判断沙箱模式是否受支持。
func IsKnownSandboxMode(mode SandboxMode) bool {
	switch mode {
	case SandboxModeReadOnly, SandboxModeWorkspaceWrite, SandboxModeDangerFullAccess:
		return true
	default:
		return false
	}
}

// IsKnownApprovalMode reports whether the approval mode is supported.
// IsKnownApprovalMode 判断审批模式是否受支持。
func IsKnownApprovalMode(mode ApprovalMode) bool {
	switch mode {
	case ApprovalModeInteractive, ApprovalModeAuto, ApprovalModeNever:
		return true
	default:
		return false
	}
}

// DefaultContextForSession creates the recommended baseline context for a mode.
// DefaultContextForSession 为指定模式创建推荐的基础上下文。
func DefaultContextForSession(sessionID, workspaceRoot, dataDir string, mode SessionMode) (Context, error) {
	ctx := Context{
		SessionID:     strings.TrimSpace(sessionID),
		WorkspaceRoot: strings.TrimSpace(workspaceRoot),
		DataDir:       strings.TrimSpace(dataDir),
		SessionMode:   mode,
	}
	switch mode {
	case SessionModeAsk:
		ctx.SandboxMode = SandboxModeReadOnly
		ctx.ApprovalMode = ApprovalModeNever
	case SessionModePlan:
		ctx.SandboxMode = SandboxModeReadOnly
		ctx.ApprovalMode = ApprovalModeInteractive
	case SessionModeAgent:
		ctx.SandboxMode = SandboxModeWorkspaceWrite
		ctx.ApprovalMode = ApprovalModeInteractive
	default:
		return Context{}, fmt.Errorf("unsupported session mode %q", mode)
	}
	if err := ctx.Validate(); err != nil {
		return Context{}, err
	}
	return ctx, nil
}
