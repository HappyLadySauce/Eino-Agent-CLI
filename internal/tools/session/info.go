package session

import (
	"context"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"
)

func init() {
	register(core.GenericDefinition[EmptyInput, InfoOutput]{
		Descriptor:  security.ToolDescriptor{Name: "session_info", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindMemory, Risk: security.OperationRiskLow, AutoApprove: true},
		Description: "Return current security session metadata.",
		Request: func(core.Runtime, EmptyInput) security.OperationRequest {
			return security.OperationRequest{Operation: security.OperationRead, Risk: security.OperationRiskLow}
		},
		Handler: func(_ context.Context, runtime core.Runtime, _ EmptyInput) (*InfoOutput, error) {
			return &InfoOutput{
				SessionID: runtime.Options.Context.SessionID,
				Mode:      string(runtime.Options.Context.SessionMode),
				Sandbox:   string(runtime.Options.Context.SandboxMode),
				Approval:  string(runtime.Options.Context.ApprovalMode),
			}, nil
		},
	})
}
