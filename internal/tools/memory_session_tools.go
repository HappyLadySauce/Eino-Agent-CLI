package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
)

func (f secureToolFactory) listMemoryTool() (einotool.BaseTool, error) {
	return secureInfer[ListAgentsInput, MemoryOutput](f,
		security.ToolDescriptor{Name: "list_memory", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindMemory, Risk: security.OperationRiskLow, AutoApprove: true},
		"List saved memory entry names.",
		func(ListAgentsInput) security.OperationRequest {
			return security.OperationRequest{Operation: security.OperationRead, Risk: security.OperationRiskLow}
		},
		func(ListAgentsInput) []string { return nil },
		func(context.Context, ListAgentsInput) (*MemoryOutput, error) {
			dir := filepath.Join(f.opts.Context.DataDir, "memory")
			entries, err := os.ReadDir(dir)
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
	)
}

func (f secureToolFactory) readMemoryTool() (einotool.BaseTool, error) {
	return secureInfer[MemoryInput, MemoryOutput](f,
		security.ToolDescriptor{Name: "read_memory", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindMemory, Risk: security.OperationRiskLow, AutoApprove: true},
		"Read one saved memory entry.",
		func(input MemoryInput) security.OperationRequest {
			return security.OperationRequest{Operation: security.OperationRead, TargetPath: filepath.Join(f.opts.Context.DataDir, "memory", safeName(input.Name)+".md"), Risk: security.OperationRiskLow}
		},
		func(MemoryInput) []string { return nil },
		func(_ context.Context, input MemoryInput) (*MemoryOutput, error) {
			path := filepath.Join(f.opts.Context.DataDir, "memory", safeName(input.Name)+".md")
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read memory: %w", err)
			}
			return &MemoryOutput{Name: input.Name, Content: string(data)}, nil
		},
	)
}

func (f secureToolFactory) writeMemoryTool() (einotool.BaseTool, error) {
	return secureInfer[MemoryInput, MemoryOutput](f,
		security.ToolDescriptor{Name: "write_memory", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindMemory, Risk: security.OperationRiskMedium, SupportsDryRun: true},
		"Write one memory entry under the Eino data directory.",
		func(input MemoryInput) security.OperationRequest {
			return security.OperationRequest{Operation: security.OperationMemoryWrite, TargetPath: filepath.Join(f.opts.Context.DataDir, "memory", safeName(input.Name)+".md"), Risk: security.OperationRiskMedium}
		},
		func(MemoryInput) []string { return nil },
		func(_ context.Context, input MemoryInput) (*MemoryOutput, error) {
			if input.DryRun {
				return &MemoryOutput{Name: input.Name, Changed: false, Message: "dry run: memory would be written"}, nil
			}
			dir := filepath.Join(f.opts.Context.DataDir, "memory")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return nil, fmt.Errorf("create memory directory: %w", err)
			}
			path := filepath.Join(dir, safeName(input.Name)+".md")
			if err := os.WriteFile(path, []byte(input.Content), 0o600); err != nil {
				return nil, fmt.Errorf("write memory: %w", err)
			}
			return &MemoryOutput{Name: input.Name, Changed: true, Message: "memory written"}, nil
		},
	)
}

func (f secureToolFactory) sessionInfoTool() (einotool.BaseTool, error) {
	return secureInfer[ListAgentsInput, SessionOutput](f,
		security.ToolDescriptor{Name: "session_info", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindMemory, Risk: security.OperationRiskLow, AutoApprove: true},
		"Return current security session metadata.",
		func(ListAgentsInput) security.OperationRequest {
			return security.OperationRequest{Operation: security.OperationRead, Risk: security.OperationRiskLow}
		},
		func(ListAgentsInput) []string { return nil },
		func(context.Context, ListAgentsInput) (*SessionOutput, error) {
			return &SessionOutput{
				SessionID: f.opts.Context.SessionID,
				Mode:      string(f.opts.Context.SessionMode),
				Sandbox:   string(f.opts.Context.SandboxMode),
				Approval:  string(f.opts.Context.ApprovalMode),
			}, nil
		},
	)
}

// SaveSessionMetadata writes a minimal session metadata file.
// SaveSessionMetadata 写入最小会话元数据文件。
func SaveSessionMetadata(secCtx security.Context) error {
	dir := filepath.Join(secCtx.DataDir, "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create sessions directory: %w", err)
	}
	payload := SessionMetadata{
		SessionID: secCtx.SessionID,
		Mode:      string(secCtx.SessionMode),
		Sandbox:   string(secCtx.SandboxMode),
		Approval:  string(secCtx.ApprovalMode),
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session metadata: %w", err)
	}
	path := filepath.Join(dir, secCtx.SessionID+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write session metadata: %w", err)
	}
	return nil
}

// LoadLatestSessionMetadata reads the newest persisted session metadata.
// LoadLatestSessionMetadata 读取最新持久化会话元数据。
func LoadLatestSessionMetadata(dataDir string) (*SessionMetadata, error) {
	dir := filepath.Join(dataDir, "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list session metadata: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		infoI, errI := entries[i].Info()
		infoJ, errJ := entries[j].Info()
		if errI != nil || errJ != nil {
			return entries[i].Name() > entries[j].Name()
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read session metadata: %w", err)
		}
		var metadata SessionMetadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			return nil, fmt.Errorf("decode session metadata: %w", err)
		}
		return &metadata, nil
	}
	return nil, nil
}
