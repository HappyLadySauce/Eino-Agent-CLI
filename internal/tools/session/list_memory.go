package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"
)

func init() {
	register(core.GenericDefinition[EmptyInput, MemoryOutput]{
		Descriptor:  security.ToolDescriptor{Name: "list_memory", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindMemory, Risk: security.OperationRiskLow, AutoApprove: true},
		Description: "List saved memory entry names.",
		Request: func(core.Runtime, EmptyInput) security.OperationRequest {
			return security.OperationRequest{Operation: security.OperationRead, Risk: security.OperationRiskLow}
		},
		Handler: func(_ context.Context, runtime core.Runtime, _ EmptyInput) (*MemoryOutput, error) {
			entries, err := os.ReadDir(memoryDir(runtime))
			if err != nil {
				if os.IsNotExist(err) {
					return &MemoryOutput{}, nil
				}
				return nil, fmt.Errorf("list memory: %w", err)
			}
			names := make([]string, 0, len(entries))
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
					names = append(names, strings.TrimSuffix(entry.Name(), ".md"))
				}
			}
			return &MemoryOutput{Names: names}, nil
		},
	})
}

func memoryDir(runtime core.Runtime) string {
	return filepath.Join(runtime.Options.Context.DataDir, "memory")
}
