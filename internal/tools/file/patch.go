package file

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"
)

func init() {
	register(core.GenericDefinition[PatchFileInput, MutationOutput]{
		Descriptor:  security.ToolDescriptor{Name: "patch_file", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindFileWrite, Risk: security.OperationRiskLow, SupportsDryRun: true},
		Description: "Replace text inside a workspace file when old_text appears exactly once.",
		Request: func(runtime core.Runtime, input PatchFileInput) security.OperationRequest {
			resolved, risk := core.ResolvePathRequest(runtime, input.Path, security.OperationWrite)
			return security.OperationRequest{Operation: security.OperationWrite, TargetPath: resolved, Risk: risk}
		},
		Handler: func(_ context.Context, runtime core.Runtime, input PatchFileInput) (*MutationOutput, error) {
			path, err := core.ResolvePath(runtime, input.Path, security.OperationWrite)
			if err != nil {
				return nil, err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read file: %w", err)
			}
			content := string(data)
			if strings.Count(content, input.OldText) != 1 {
				return nil, fmt.Errorf("old_text must appear exactly once")
			}
			if input.DryRun {
				return &MutationOutput{Path: path, Changed: false, Message: "dry run: patch would be applied"}, nil
			}
			next := strings.Replace(content, input.OldText, input.NewText, 1)
			if err := atomicWrite(path, []byte(next)); err != nil {
				return nil, err
			}
			return &MutationOutput{Path: path, Changed: true, Message: "patch applied"}, nil
		},
	})
}
