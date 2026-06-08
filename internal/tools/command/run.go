package command

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"
)

const defaultMaxOutputBytes int64 = 64 * 1024

func init() {
	register(core.GenericDefinition[RunInput, RunOutput]{
		Descriptor:  security.ToolDescriptor{Name: "run_command", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindCommand, Risk: security.OperationRiskMedium, SupportsDryRun: true, RateLimit: &security.RateLimitDescriptor{MaxCallsPerMinute: 20}},
		Description: "Run a local command through the security policy.",
		Request: func(runtime core.Runtime, input RunInput) security.OperationRequest {
			classification := security.ClassifyCommand(input.Command)
			cwd := commandCWD(runtime, input.CWD)
			risk := classification.Risk
			if risk == "" {
				risk = security.OperationRiskUnknown
			}
			if _, pathRisk := core.ResolvePathRequest(runtime, cwd, security.OperationRead); pathRisk == security.OperationRiskUnknown {
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
		Tokens: func(_ core.Runtime, input RunInput) []string {
			return security.ClassifyCommand(input.Command).Tokens
		},
		Handler: func(ctx context.Context, runtime core.Runtime, input RunInput) (*RunOutput, error) {
			classification := security.ClassifyCommand(input.Command)
			cwd := commandCWD(runtime, input.CWD)
			resolvedCWD, err := core.ResolvePath(runtime, cwd, security.OperationRead)
			if err != nil {
				return nil, err
			}
			if input.DryRun {
				return &RunOutput{Command: input.Command, Risk: string(classification.Risk), DryRun: true}, nil
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
					return &RunOutput{Command: input.Command, ExitCode: -1, Stderr: runCtx.Err().Error(), Risk: string(classification.Risk)}, nil
				} else {
					return nil, fmt.Errorf("run command: %w", err)
				}
			}
			return &RunOutput{
				Command:   input.Command,
				ExitCode:  exitCode,
				Stdout:    stdout.String(),
				Stderr:    stderr.String(),
				Truncated: stdoutLimit.truncated || stderrLimit.truncated,
				Risk:      string(classification.Risk),
			}, nil
		},
	})
}

func commandCWD(runtime core.Runtime, cwd string) string {
	if strings.TrimSpace(cwd) == "" {
		return runtime.Options.Context.WorkspaceRoot
	}
	return cwd
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
