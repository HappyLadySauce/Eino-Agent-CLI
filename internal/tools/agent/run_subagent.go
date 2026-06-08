package agent

import (
	"context"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"
)

func init() {
	register(core.GenericDefinition[RunSubAgentInput, RunSubAgentOutput]{
		Descriptor: security.ToolDescriptor{
			Name:     "run_subagent",
			Provider: security.ToolProviderBuiltin,
			Kind:     security.ToolKindAgent,
			Risk:     security.OperationRiskMedium,
		},
		Description: "Run one named sub-agent with a fresh isolated context. agent_name is required.",
		Enable: func(runtime core.Runtime) bool {
			return runtime.Mode != string(security.SessionModeAsk)
		},
		Request: func(core.Runtime, RunSubAgentInput) security.OperationRequest {
			return security.OperationRequest{Operation: security.OperationExec, Risk: security.OperationRiskMedium}
		},
		Handler: func(ctx context.Context, runtime core.Runtime, input RunSubAgentInput) (*RunSubAgentOutput, error) {
			svc, err := service(runtime)
			if err != nil {
				return nil, err
			}
			return svc.RunSubAgent(ctx, runtime.Mode, input)
		},
	})
}
