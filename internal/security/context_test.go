package security

import (
	"strings"
	"testing"
)

func TestDefaultContextForSession(t *testing.T) {
	tests := []struct {
		name     string
		mode     SessionMode
		sandbox  SandboxMode
		approval ApprovalMode
	}{
		{name: "ask", mode: SessionModeAsk, sandbox: SandboxModeReadOnly, approval: ApprovalModeNever},
		{name: "plan", mode: SessionModePlan, sandbox: SandboxModeReadOnly, approval: ApprovalModeInteractive},
		{name: "agent", mode: SessionModeAgent, sandbox: SandboxModeWorkspaceWrite, approval: ApprovalModeInteractive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := DefaultContextForSession("session-1", t.TempDir(), t.TempDir(), tt.mode)
			if err != nil {
				t.Fatalf("DefaultContextForSession() error = %v", err)
			}
			if ctx.SandboxMode != tt.sandbox {
				t.Fatalf("SandboxMode = %q, want %q", ctx.SandboxMode, tt.sandbox)
			}
			if ctx.ApprovalMode != tt.approval {
				t.Fatalf("ApprovalMode = %q, want %q", ctx.ApprovalMode, tt.approval)
			}
		})
	}
}

func TestContextValidateRejectsInvalidCombinations(t *testing.T) {
	tests := []struct {
		name string
		ctx  Context
		want string
	}{
		{
			name: "ask workspace write",
			ctx: Context{
				SessionID:     "session-1",
				WorkspaceRoot: t.TempDir(),
				DataDir:       t.TempDir(),
				SessionMode:   SessionModeAsk,
				SandboxMode:   SandboxModeWorkspaceWrite,
				ApprovalMode:  ApprovalModeNever,
			},
			want: "ask mode requires read-only sandbox",
		},
		{
			name: "plan auto",
			ctx: Context{
				SessionID:     "session-1",
				WorkspaceRoot: t.TempDir(),
				DataDir:       t.TempDir(),
				SessionMode:   SessionModePlan,
				SandboxMode:   SandboxModeReadOnly,
				ApprovalMode:  ApprovalModeAuto,
			},
			want: "plan mode cannot use auto approval",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ctx.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
