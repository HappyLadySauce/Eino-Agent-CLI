package file

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"
)

func init() {
	register(core.GenericDefinition[ReadFileInput, ReadFileOutput]{
		Descriptor:  security.ToolDescriptor{Name: "read_file", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindFileRead, Risk: security.OperationRiskLow, AutoApprove: true},
		Description: "Read a UTF-8 file inside the workspace.",
		Request: func(runtime core.Runtime, input ReadFileInput) security.OperationRequest {
			resolved, risk := core.ResolvePathRequest(runtime, input.Path, security.OperationRead)
			return security.OperationRequest{Operation: security.OperationRead, TargetPath: resolved, Risk: risk}
		},
		Handler: func(_ context.Context, runtime core.Runtime, input ReadFileInput) (*ReadFileOutput, error) {
			path, err := core.ResolvePath(runtime, input.Path, security.OperationRead)
			if err != nil {
				return nil, err
			}
			maxBytes := input.MaxBytes
			if maxBytes <= 0 {
				maxBytes = defaultMaxOutputBytes
			}
			file, err := os.Open(path)
			if err != nil {
				return nil, fmt.Errorf("open file: %w", err)
			}
			defer file.Close()
			data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
			if err != nil {
				return nil, fmt.Errorf("read file: %w", err)
			}
			truncated := int64(len(data)) > maxBytes
			if truncated {
				data = data[:maxBytes]
			}
			return &ReadFileOutput{Path: path, Content: string(data), Truncated: truncated}, nil
		},
	})
}
