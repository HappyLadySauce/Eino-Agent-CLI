package approval

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
)

// Request contains the context shown to a user before a sensitive operation.
// Request 包含敏感操作执行前展示给用户的上下文。
type Request struct {
	ToolName string
	CWD      string
	Input    string
	Risk     security.OperationRisk
	Reason   string
}

// Decision is the user's approval response.
// Decision 是用户的审批响应。
type Decision string

const (
	DecisionApproveOnce Decision = "approve_once"
	DecisionDeny        Decision = "deny"
)

// Prompter asks the user whether a sensitive operation may run.
// Prompter 询问用户是否允许执行敏感操作。
type Prompter interface {
	Prompt(ctx context.Context, request Request) (Decision, error)
}

// CLIPropter implements interactive terminal approval.
// CLIPropter 实现交互式终端审批。
type CLIPropter struct {
	reader LineReader
	writer io.Writer
}

// NewCLIPrompter creates a CLI approval prompter.
// NewCLIPrompter 创建 CLI 审批器。
func NewCLIPrompter(reader LineReader, writer io.Writer) *CLIPropter {
	return &CLIPropter{
		reader: reader,
		writer: writer,
	}
}

// Prompt asks for approve-once or deny.
// Prompt 询问本次允许或拒绝。
func (p *CLIPropter) Prompt(ctx context.Context, request Request) (Decision, error) {
	if p == nil || p.reader == nil || p.writer == nil {
		return DecisionDeny, fmt.Errorf("approval prompter is not configured")
	}
	select {
	case <-ctx.Done():
		return DecisionDeny, ctx.Err()
	default:
	}
	fmt.Fprintf(p.writer, "\nTool: %s\n", request.ToolName)
	if strings.TrimSpace(request.CWD) != "" {
		fmt.Fprintf(p.writer, "CWD: %s\n", request.CWD)
	}
	if strings.TrimSpace(request.Input) != "" {
		fmt.Fprintf(p.writer, "Input: %s\n", request.Input)
	}
	p.reader.BeginApproval()
	defer p.reader.EndApproval()

	fmt.Fprintf(p.writer, "Risk: %s\nReason: %s\nDecision: approve once? [y/N] ", request.Risk, request.Reason)
	line, err := p.reader.ReadApprovalLine(ctx)
	if err != nil {
		return DecisionDeny, fmt.Errorf("read approval decision: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return DecisionApproveOnce, nil
	default:
		return DecisionDeny, nil
	}
}

// FakePrompter is used by tests.
// FakePrompter 用于测试。
type FakePrompter struct {
	Decision Decision
	Err      error
	Requests []Request
}

// Prompt records the request and returns the configured decision.
// Prompt 记录请求并返回配置的决策。
func (p *FakePrompter) Prompt(_ context.Context, request Request) (Decision, error) {
	if p == nil {
		return DecisionDeny, nil
	}
	p.Requests = append(p.Requests, request)
	if p.Err != nil {
		return DecisionDeny, p.Err
	}
	if p.Decision == "" {
		return DecisionDeny, nil
	}
	return p.Decision, nil
}
