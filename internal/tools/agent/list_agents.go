package agent

import (
	"context"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"
)

func init() {
	register(core.GenericDefinition[ListAgentsInput, ListAgentsOutput]{
		Descriptor: security.ToolDescriptor{
			Name:        "list_agents",
			Provider:    security.ToolProviderBuiltin,
			Kind:        security.ToolKindAgent,
			Risk:        security.OperationRiskLow,
			AutoApprove: true,
		},
		Description: "List available dynamically created sub-agents with their permissions.",
		Request: func(core.Runtime, ListAgentsInput) security.OperationRequest {
			return security.OperationRequest{Operation: security.OperationRead, Risk: security.OperationRiskLow}
		},
		Handler: func(ctx context.Context, runtime core.Runtime, input ListAgentsInput) (*ListAgentsOutput, error) {
			svc, err := service(runtime)
			if err != nil {
				return nil, err
			}
			return svc.ListAgents(ctx, runtime.Mode, input)
		},
	})
}
