package file

import (
	"context"
	"fmt"
	"os"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"
)

func init() {
	register(core.GenericDefinition[ListDirInput, ListDirOutput]{
		Descriptor:  security.ToolDescriptor{Name: "list_dir", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindFileRead, Risk: security.OperationRiskLow, AutoApprove: true},
		Description: "List a directory inside the workspace.",
		Request: func(runtime core.Runtime, input ListDirInput) security.OperationRequest {
			resolved, risk := core.ResolvePathRequest(runtime, input.Path, security.OperationList)
			return security.OperationRequest{Operation: security.OperationList, TargetPath: resolved, Risk: risk}
		},
		Handler: func(_ context.Context, runtime core.Runtime, input ListDirInput) (*ListDirOutput, error) {
			path, err := core.ResolvePath(runtime, input.Path, security.OperationList)
			if err != nil {
				return nil, err
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil, fmt.Errorf("list directory: %w", err)
			}
			out := make([]DirEntry, 0, len(entries))
			for _, entry := range entries {
				info, err := entry.Info()
				if err != nil {
					return nil, fmt.Errorf("read directory entry: %w", err)
				}
				out = append(out, DirEntry{Name: entry.Name(), IsDir: entry.IsDir(), Size: info.Size()})
			}
			return &ListDirOutput{Path: path, Entries: out}, nil
		},
	})
}
