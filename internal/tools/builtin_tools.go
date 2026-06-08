package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
)

const defaultMaxOutputBytes int64 = 64 * 1024

func (f secureToolFactory) listAgentsTool() (einotool.BaseTool, error) {
	return secureInfer[ListAgentsInput, ListAgentsOutput](f,
		security.ToolDescriptor{Name: "list_agents", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindAgent, Risk: security.OperationRiskLow, AutoApprove: true},
		"List available dynamically created sub-agents with their permissions.",
		func(ListAgentsInput) security.OperationRequest {
			return security.OperationRequest{Operation: security.OperationRead, Risk: security.OperationRiskLow}
		},
		func(ListAgentsInput) []string { return nil },
		func(ctx context.Context, input ListAgentsInput) (*ListAgentsOutput, error) {
			return f.service.ListAgents(ctx, f.mode, input)
		},
	)
}

func (f secureToolFactory) createAgentTool() (einotool.BaseTool, error) {
	return secureInfer[CreateAgentInput, CreateAgentOutput](f,
		security.ToolDescriptor{Name: "create_agent", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindAgent, Risk: security.OperationRiskMedium},
		"Create a new in-memory sub-agent. permission_mode is required. In plan mode only readonly or plan agents are allowed.",
		func(CreateAgentInput) security.OperationRequest {
			return security.OperationRequest{Operation: security.OperationWrite, Risk: security.OperationRiskMedium}
		},
		func(CreateAgentInput) []string { return nil },
		func(ctx context.Context, input CreateAgentInput) (*CreateAgentOutput, error) {
			return f.service.CreateAgent(ctx, f.mode, input)
		},
	)
}

func (f secureToolFactory) runSubAgentTool() (einotool.BaseTool, error) {
	return secureInfer[RunSubAgentInput, RunSubAgentOutput](f,
		security.ToolDescriptor{Name: "run_subagent", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindAgent, Risk: security.OperationRiskMedium},
		"Run one named sub-agent with a fresh isolated context. agent_name is required.",
		func(RunSubAgentInput) security.OperationRequest {
			return security.OperationRequest{Operation: security.OperationExec, Risk: security.OperationRiskMedium}
		},
		func(RunSubAgentInput) []string { return nil },
		func(ctx context.Context, input RunSubAgentInput) (*RunSubAgentOutput, error) {
			return f.service.RunSubAgent(ctx, f.mode, input)
		},
	)
}

func (f secureToolFactory) readFileTool() (einotool.BaseTool, error) {
	return secureInfer[ReadFileInput, ReadFileOutput](f,
		security.ToolDescriptor{Name: "read_file", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindFileRead, Risk: security.OperationRiskLow, AutoApprove: true},
		"Read a UTF-8 file inside the workspace.",
		func(input ReadFileInput) security.OperationRequest {
			resolved, risk := f.resolvePathRequest(input.Path, security.OperationRead)
			return security.OperationRequest{Operation: security.OperationRead, TargetPath: resolved, Risk: risk}
		},
		func(ReadFileInput) []string { return nil },
		func(_ context.Context, input ReadFileInput) (*ReadFileOutput, error) {
			path, err := f.resolvePath(input.Path, security.OperationRead)
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
			limited := io.LimitReader(file, maxBytes+1)
			data, err := io.ReadAll(limited)
			if err != nil {
				return nil, fmt.Errorf("read file: %w", err)
			}
			truncated := int64(len(data)) > maxBytes
			if truncated {
				data = data[:maxBytes]
			}
			return &ReadFileOutput{Path: path, Content: string(data), Truncated: truncated}, nil
		},
	)
}

func (f secureToolFactory) listDirTool() (einotool.BaseTool, error) {
	return secureInfer[ListDirInput, ListDirOutput](f,
		security.ToolDescriptor{Name: "list_dir", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindFileRead, Risk: security.OperationRiskLow, AutoApprove: true},
		"List a directory inside the workspace.",
		func(input ListDirInput) security.OperationRequest {
			resolved, risk := f.resolvePathRequest(input.Path, security.OperationList)
			return security.OperationRequest{Operation: security.OperationList, TargetPath: resolved, Risk: risk}
		},
		func(ListDirInput) []string { return nil },
		func(_ context.Context, input ListDirInput) (*ListDirOutput, error) {
			path, err := f.resolvePath(input.Path, security.OperationList)
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
	)
}

func (f secureToolFactory) createFileTool() (einotool.BaseTool, error) {
	return f.writeFileTool("create_file", "Create a new UTF-8 file inside the workspace.", false)
}

func (f secureToolFactory) replaceFileTool() (einotool.BaseTool, error) {
	return f.writeFileTool("replace_file", "Replace a UTF-8 file inside the workspace.", true)
}

func (f secureToolFactory) writeFileTool(name, desc string, allowExisting bool) (einotool.BaseTool, error) {
	return secureInfer[WriteFileInput, FileMutationOutput](f,
		security.ToolDescriptor{Name: name, Provider: security.ToolProviderBuiltin, Kind: security.ToolKindFileWrite, Risk: security.OperationRiskLow, SupportsDryRun: true},
		desc,
		func(input WriteFileInput) security.OperationRequest {
			resolved, risk := f.resolvePathRequest(input.Path, operationForFileWrite(input.DryRun))
			return security.OperationRequest{Operation: security.OperationWrite, TargetPath: resolved, Risk: risk}
		},
		func(WriteFileInput) []string { return nil },
		func(_ context.Context, input WriteFileInput) (*FileMutationOutput, error) {
			path, err := f.resolvePath(input.Path, security.OperationWrite)
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
				return &FileMutationOutput{Path: path, Changed: false, Message: "dry run: file would be written"}, nil
			}
			if err := atomicWrite(path, []byte(input.Content)); err != nil {
				return nil, err
			}
			return &FileMutationOutput{Path: path, Changed: true, Message: "file written"}, nil
		},
	)
}

func (f secureToolFactory) patchFileTool() (einotool.BaseTool, error) {
	return secureInfer[PatchFileInput, FileMutationOutput](f,
		security.ToolDescriptor{Name: "patch_file", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindFileWrite, Risk: security.OperationRiskLow, SupportsDryRun: true},
		"Replace text inside a workspace file when old_text appears exactly once.",
		func(input PatchFileInput) security.OperationRequest {
			resolved, risk := f.resolvePathRequest(input.Path, security.OperationWrite)
			return security.OperationRequest{Operation: security.OperationWrite, TargetPath: resolved, Risk: risk}
		},
		func(PatchFileInput) []string { return nil },
		func(_ context.Context, input PatchFileInput) (*FileMutationOutput, error) {
			path, err := f.resolvePath(input.Path, security.OperationWrite)
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
			next := strings.Replace(content, input.OldText, input.NewText, 1)
			if input.DryRun {
				return &FileMutationOutput{Path: path, Changed: false, Message: "dry run: patch would be applied"}, nil
			}
			if err := atomicWrite(path, []byte(next)); err != nil {
				return nil, err
			}
			return &FileMutationOutput{Path: path, Changed: true, Message: "patch applied"}, nil
		},
	)
}

func (f secureToolFactory) deleteFileTool() (einotool.BaseTool, error) {
	return secureInfer[DeleteFileInput, FileMutationOutput](f,
		security.ToolDescriptor{Name: "delete_file", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindFileDelete, Risk: security.OperationRiskDestructive, SupportsDryRun: true},
		"Delete one file inside the workspace.",
		func(input DeleteFileInput) security.OperationRequest {
			resolved, risk := f.resolvePathRequest(input.Path, security.OperationDelete)
			return security.OperationRequest{Operation: security.OperationDelete, TargetPath: resolved, Risk: risk}
		},
		func(DeleteFileInput) []string { return nil },
		func(_ context.Context, input DeleteFileInput) (*FileMutationOutput, error) {
			path, err := f.resolvePath(input.Path, security.OperationDelete)
			if err != nil {
				return nil, err
			}
			if input.DryRun {
				return &FileMutationOutput{Path: path, Changed: false, Message: "dry run: file would be deleted"}, nil
			}
			if err := os.Remove(path); err != nil {
				return nil, fmt.Errorf("delete file: %w", err)
			}
			return &FileMutationOutput{Path: path, Changed: true, Message: "file deleted"}, nil
		},
	)
}

func (f secureToolFactory) runCommandTool() (einotool.BaseTool, error) {
	return secureInfer[RunCommandInput, RunCommandOutput](f,
		security.ToolDescriptor{Name: "run_command", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindCommand, Risk: security.OperationRiskMedium, SupportsDryRun: true, RateLimit: &security.RateLimitDescriptor{MaxCallsPerMinute: 20}},
		"Run a local command through the security policy.",
		func(input RunCommandInput) security.OperationRequest {
			classification := security.ClassifyCommand(input.Command)
			cwd := input.CWD
			if strings.TrimSpace(cwd) == "" {
				cwd = f.opts.Context.WorkspaceRoot
			}
			risk := classification.Risk
			if risk == "" {
				risk = security.OperationRiskUnknown
			}
			_, pathRisk := f.resolvePathRequest(cwd, security.OperationRead)
			if pathRisk == security.OperationRiskUnknown {
				risk = security.OperationRiskUnknown
			}
			return security.OperationRequest{
				Operation: security.OperationExec,
				Command:   input.Command,
				CWD:       cwd,
				Risk:      risk,
				Network:   classification.Network,
				Unknown:   risk == security.OperationRiskUnknown,
			}
		},
		func(input RunCommandInput) []string { return security.ClassifyCommand(input.Command).Tokens },
		func(ctx context.Context, input RunCommandInput) (*RunCommandOutput, error) {
			classification := security.ClassifyCommand(input.Command)
			cwd := input.CWD
			if strings.TrimSpace(cwd) == "" {
				cwd = f.opts.Context.WorkspaceRoot
			}
			resolvedCWD, err := f.resolvePath(cwd, security.OperationRead)
			if err != nil {
				return nil, err
			}
			if input.DryRun {
				return &RunCommandOutput{Command: input.Command, Risk: string(classification.Risk), DryRun: true}, nil
			}
			timeout := time.Duration(input.TimeoutSeconds) * time.Second
			if timeout <= 0 {
				timeout = 30 * time.Second
			}
			runCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			cmd := commandForPlatform(runCtx, input.Command)
			cmd.Dir = resolvedCWD
			var stdout, stderr bytes.Buffer
			stdoutLimit := &limitWriter{writer: &stdout, limit: defaultMaxOutputBytes}
			stderrLimit := &limitWriter{writer: &stderr, limit: defaultMaxOutputBytes}
			cmd.Stdout = stdoutLimit
			cmd.Stderr = stderrLimit
			err = cmd.Run()
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else if runCtx.Err() != nil {
					return &RunCommandOutput{Command: input.Command, ExitCode: -1, Stderr: runCtx.Err().Error(), Risk: string(classification.Risk)}, nil
				} else {
					return nil, fmt.Errorf("run command: %w", err)
				}
			}
			return &RunCommandOutput{
				Command:   input.Command,
				ExitCode:  exitCode,
				Stdout:    stdout.String(),
				Stderr:    stderr.String(),
				Truncated: stdoutLimit.truncated || stderrLimit.truncated,
				Risk:      string(classification.Risk),
			}, nil
		},
	)
}

func commandForPlatform(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", command)
	}
	return exec.CommandContext(ctx, "/bin/sh", "-c", command)
}

type limitWriter struct {
	writer    io.Writer
	limit     int64
	written   int64
	truncated bool
}

func (w *limitWriter) Write(p []byte) (int, error) {
	remaining := w.limit - w.written
	if remaining <= 0 {
		w.truncated = true
		return len(p), nil
	}
	originalLen := len(p)
	if int64(len(p)) > remaining {
		p = p[:remaining]
		w.truncated = true
	}
	n, err := w.writer.Write(p)
	w.written += int64(n)
	return originalLen, err
}

func (f secureToolFactory) resolvePath(path string, operation security.OperationKind) (string, error) {
	resolved, err := security.ResolveWorkspacePath(f.opts.Context.WorkspaceRoot, path, operation)
	if err != nil {
		return "", err
	}
	return resolved.Absolute, nil
}

func (f secureToolFactory) resolvePathRequest(path string, operation security.OperationKind) (string, security.OperationRisk) {
	resolved, err := security.ResolveWorkspacePath(f.opts.Context.WorkspaceRoot, path, operation)
	if err != nil {
		return path, security.OperationRiskUnknown
	}
	return resolved.Absolute, security.OperationRiskLow
}

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}
	return nil
}
