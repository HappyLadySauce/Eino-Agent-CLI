package file

import (
	"context"
	"fmt"
	"os"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"
)

func writeDefinition(name, description string, allowExisting bool) core.Definition {
	return core.GenericDefinition[WriteFileInput, MutationOutput]{
		Descriptor:  security.ToolDescriptor{Name: name, Provider: security.ToolProviderBuiltin, Kind: security.ToolKindFileWrite, Risk: security.OperationRiskLow, SupportsDryRun: true},
		Description: description,
		Request: func(runtime core.Runtime, input WriteFileInput) security.OperationRequest {
			resolved, risk := core.ResolvePathRequest(runtime, input.Path, core.OperationForFileWrite(input.DryRun))
			return security.OperationRequest{Operation: security.OperationWrite, TargetPath: resolved, Risk: risk}
		},
		Handler: func(_ context.Context, runtime core.Runtime, input WriteFileInput) (*MutationOutput, error) {
			path, err := core.ResolvePath(runtime, input.Path, security.OperationWrite)
			if err != nil {
				return nil, err
			}
			_, statErr := os.Stat(path)
			if !allowExisting && statErr == nil {
				return nil, fmt.Errorf("file already exists")
			}
			if allowExisting && statErr != nil && !os.IsNotExist(statErr) {
				return nil, fmt.Errorf("stat file: %w", statErr)
			}
			if input.DryRun {
				return &MutationOutput{Path: path, Changed: false, Message: "dry run: file would be written"}, nil
			}
			if err := atomicWrite(path, []byte(input.Content)); err != nil {
				return nil, err
			}
			return &MutationOutput{Path: path, Changed: true, Message: "file written"}, nil
		},
	}
}
