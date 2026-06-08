package agent

import (
	"context"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"
)

func init() {
	register(core.GenericDefinition[CreateAgentInput, CreateAgentOutput]{
		Descriptor: security.ToolDescriptor{
			Name:     "create_agent",
			Provider: security.ToolProviderBuiltin,
			Kind:     security.ToolKindAgent,
			Risk:     security.OperationRiskMedium,
		},
		Description: "Create a new in-memory sub-agent. permission_mode is required. In plan mode only readonly or plan agents are allowed.",
		Enable: func(runtime core.Runtime) bool {
			return runtime.Mode != string(security.SessionModeAsk)
		},
		Request: func(core.Runtime, CreateAgentInput) security.OperationRequest {
			return security.OperationRequest{Operation: security.OperationWrite, Risk: security.OperationRiskMedium}
		},
		Handler: func(ctx context.Context, runtime core.Runtime, input CreateAgentInput) (*CreateAgentOutput, error) {
			svc, err := service(runtime)
			if err != nil {
				return nil, err
			}
			return svc.CreateAgent(ctx, runtime.Mode, input)
		},
	})
}
