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
		Descriptor:  security.ToolDescriptor{Name: "write_memory", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindMemory, Risk: security.OperationRiskMedium, SupportsDryRun: true},
		Description: "Write one memory entry under the Eino data directory.",
		Request: func(runtime core.Runtime, input MemoryInput) security.OperationRequest {
			return security.OperationRequest{Operation: security.OperationMemoryWrite, TargetPath: memoryPath(runtime, input.Name), Risk: security.OperationRiskMedium}
		},
		Handler: func(_ context.Context, runtime core.Runtime, input MemoryInput) (*MemoryOutput, error) {
			if input.DryRun {
				return &MemoryOutput{Name: input.Name, Changed: false, Message: "dry run: memory would be written"}, nil
			}
			if err := os.MkdirAll(memoryDir(runtime), 0o700); err != nil {
				return nil, fmt.Errorf("create memory directory: %w", err)
			}
			if err := os.WriteFile(memoryPath(runtime, input.Name), []byte(input.Content), 0o600); err != nil {
				return nil, fmt.Errorf("write memory: %w", err)
			}
			return &MemoryOutput{Name: input.Name, Changed: true, Message: "memory written"}, nil
		},
	})
}
