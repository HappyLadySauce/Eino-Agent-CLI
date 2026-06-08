package session

import (
	"context"
	"fmt"
	"os"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"
)

func init() {
	register(core.GenericDefinition[MemoryInput, MemoryOutput]{
		Descriptor:  security.ToolDescriptor{Name: "read_memory", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindMemory, Risk: security.OperationRiskLow, AutoApprove: true},
		Description: "Read one saved memory entry.",
		Request: func(runtime core.Runtime, input MemoryInput) security.OperationRequest {
			return security.OperationRequest{Operation: security.OperationRead, TargetPath: memoryPath(runtime, input.Name), Risk: security.OperationRiskLow}
		},
		Handler: func(_ context.Context, runtime core.Runtime, input MemoryInput) (*MemoryOutput, error) {
			data, err := os.ReadFile(memoryPath(runtime, input.Name))
			if err != nil {
				return nil, fmt.Errorf("read memory: %w", err)
			}
			return &MemoryOutput{Name: input.Name, Content: string(data)}, nil
		},
	})
}
