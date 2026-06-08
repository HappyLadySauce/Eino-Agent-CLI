package security

import "testing"

func TestBuiltInDecisionMatrix(t *testing.T) {
	workspace := t.TempDir()
	dataDir := t.TempDir()
	readTool := ToolDescriptor{Name: "read_file", Provider: ToolProviderBuiltin, Kind: ToolKindFileRead, Risk: OperationRiskLow}
	writeTool := ToolDescriptor{Name: "create_file", Provider: ToolProviderBuiltin, Kind: ToolKindFileWrite, Risk: OperationRiskLow}
	commandTool := ToolDescriptor{Name: "run_command", Provider: ToolProviderBuiltin, Kind: ToolKindCommand, Risk: OperationRiskLow, AutoApprove: true}
	externalTool := ToolDescriptor{
		Name:      "mcp_call",
		Provider:  ToolProviderMCP,
		Kind:      ToolKindExternalState,
		Risk:      OperationRiskMedium,
		Resources: []ResourceDescriptor{{Kind: "domain", Targets: []string{"api.example.com"}}},
	}

	tests := []struct {
		name string
		ctx  Context
		req  OperationRequest
		want Decision
	}{
		{
			name: "ask allows low risk read",
			ctx:  mustDefaultContext(t, "s1", workspace, dataDir, SessionModeAsk),
			req:  OperationRequest{Tool: readTool, Operation: OperationRead},
			want: DecisionAllow,
		},
		{
			name: "ask denies write",
			ctx:  mustDefaultContext(t, "s1", workspace, dataDir, SessionModeAsk),
			req:  OperationRequest{Tool: writeTool, Operation: OperationWrite},
			want: DecisionDeny,
		},
		{
			name: "plan suggests low risk write",
			ctx:  mustDefaultContext(t, "s1", workspace, dataDir, SessionModePlan),
			req:  OperationRequest{Tool: writeTool, Operation: OperationWrite},
			want: DecisionSuggest,
		},
		{
			name: "plan allows low risk command",
			ctx:  mustDefaultContext(t, "s1", workspace, dataDir, SessionModePlan),
			req:  OperationRequest{Tool: commandTool, Operation: OperationExec},
			want: DecisionAllow,
		},
		{
			name: "agent makes low risk write auto eligible",
			ctx:  mustDefaultContext(t, "s1", workspace, dataDir, SessionModeAgent),
			req:  OperationRequest{Tool: writeTool, Operation: OperationWrite},
			want: DecisionAutoEligible,
		},
		{
			name: "agent asks external declared resource",
			ctx:  mustDefaultContext(t, "s1", workspace, dataDir, SessionModeAgent),
			req:  OperationRequest{Tool: externalTool, Operation: OperationExternalState, External: true},
			want: DecisionAsk,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuiltInDecision(tt.ctx, tt.req)
			if got.Decision != tt.want {
				t.Fatalf("BuiltInDecision() = %q (%s), want %q", got.Decision, got.Reason, tt.want)
			}
		})
	}
}

func TestApplyApprovalMode(t *testing.T) {
	decision := PolicyDecision{Decision: DecisionAutoEligible, Reason: "auto eligible"}
	if got := ApplyApprovalMode(ApprovalModeAuto, decision); !got.OK || got.Status != ResultStatusOK {
		t.Fatalf("ApplyApprovalMode(auto) = %+v, want ok", got)
	}
	if got := ApplyApprovalMode(ApprovalModeInteractive, decision); got.OK || got.Status != ResultStatusApprovalRequired {
		t.Fatalf("ApplyApprovalMode(interactive) = %+v, want approval_required", got)
	}
	if got := ApplyApprovalMode(ApprovalModeNever, decision); got.OK || got.Status != ResultStatusDenied {
		t.Fatalf("ApplyApprovalMode(never) = %+v, want denied", got)
	}
}

func mustDefaultContext(t *testing.T, sessionID, workspace, dataDir string, mode SessionMode) Context {
	t.Helper()
	ctx, err := DefaultContextForSession(sessionID, workspace, dataDir, mode)
	if err != nil {
		t.Fatalf("DefaultContextForSession() error = %v", err)
	}
	return ctx
}
