package file

import (
	"context"
	"fmt"
	"os"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"
)

func init() {
	register(core.GenericDefinition[DeleteFileInput, MutationOutput]{
		Descriptor:  security.ToolDescriptor{Name: "delete_file", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindFileDelete, Risk: security.OperationRiskDestructive, SupportsDryRun: true},
		Description: "Delete one file inside the workspace.",
		Request: func(runtime core.Runtime, input DeleteFileInput) security.OperationRequest {
			resolved, risk := core.ResolvePathRequest(runtime, input.Path, security.OperationDelete)
			return security.OperationRequest{Operation: security.OperationDelete, TargetPath: resolved, Risk: risk}
		},
		Handler: func(_ context.Context, runtime core.Runtime, input DeleteFileInput) (*MutationOutput, error) {
			path, err := core.ResolvePath(runtime, input.Path, security.OperationDelete)
			if err != nil {
				return nil, err
			}
			if input.DryRun {
				return &MutationOutput{Path: path, Changed: false, Message: "dry run: file would be deleted"}, nil
			}
			if err := os.Remove(path); err != nil {
				return nil, fmt.Errorf("delete file: %w", err)
			}
			return &MutationOutput{Path: path, Changed: true, Message: "file deleted"}, nil
		},
	})
}
