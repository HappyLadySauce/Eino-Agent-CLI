package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/approval"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/audit"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/rules"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
)

type fakeAgentToolService struct{}

func (fakeAgentToolService) ListAgents(context.Context, string, ListAgentsInput) (*ListAgentsOutput, error) {
	return &ListAgentsOutput{}, nil
}

func (fakeAgentToolService) CreateAgent(context.Context, string, CreateAgentInput) (*CreateAgentOutput, error) {
	return &CreateAgentOutput{}, nil
}

func (fakeAgentToolService) RunSubAgent(context.Context, string, RunSubAgentInput) (*RunSubAgentOutput, error) {
	return &RunSubAgentOutput{}, nil
}

func TestNewAgentTools(t *testing.T) {
	tools, err := NewAgentTools("plan", fakeAgentToolService{})
	if err != nil {
		t.Fatalf("NewAgentTools() error = %v", err)
	}
	if len(tools) < 10 {
		t.Fatalf("NewAgentTools() len = %d, want secured built-in tools", len(tools))
	}

	names := map[string]bool{}
	descriptions := map[string]string{}
	for _, tool := range tools {
		info, err := tool.Info(context.Background())
		if err != nil {
			t.Fatalf("Info() error = %v", err)
		}
		names[info.Name] = true
		descriptions[info.Name] = info.Desc
	}
	for _, name := range []string{"list_agents", "create_agent", "run_subagent", "read_file", "list_dir", "run_command", "session_info"} {
		if !names[name] {
			t.Fatalf("NewAgentTools() missing tool %q", name)
		}
	}
	if got := descriptions["list_agents"]; got == "" || containsAny(got, "built-in", "default sub-agent") {
		t.Fatalf("list_agents description = %q, want dynamic agent description", got)
	}
	if got := descriptions["create_agent"]; got == "" || !containsAny(got, "permission_mode is required") {
		t.Fatalf("create_agent description = %q, want required permission_mode guidance", got)
	}
	if got := descriptions["run_subagent"]; got == "" || !containsAny(got, "agent_name is required") {
		t.Fatalf("run_subagent description = %q, want required agent_name guidance", got)
	}
}

func TestAskModeDoesNotExposeCreateOrRunSubAgent(t *testing.T) {
	tools, err := NewAgentTools("ask", fakeAgentToolService{})
	if err != nil {
		t.Fatalf("NewAgentTools(ask) error = %v", err)
	}
	names := map[string]bool{}
	for _, tool := range tools {
		info, err := tool.Info(context.Background())
		if err != nil {
			t.Fatalf("Info() error = %v", err)
		}
		names[info.Name] = true
	}
	if names["create_agent"] || names["run_subagent"] {
		t.Fatalf("ask mode exposed create/run subagent: %+v", names)
	}
	if !names["read_file"] || !names["list_dir"] {
		t.Fatalf("ask mode missing read/list tools: %+v", names)
	}
}

func TestSecureToolSuggestedWriteInPlan(t *testing.T) {
	secCtx := security.Context{
		SessionID:     "session-1",
		WorkspaceRoot: t.TempDir(),
		DataDir:       t.TempDir(),
		SessionMode:   security.SessionModePlan,
		SandboxMode:   security.SandboxModeReadOnly,
		ApprovalMode:  security.ApprovalModeInteractive,
	}
	factory := secureToolFactory{
		mode:    "plan",
		service: fakeAgentToolService{},
		opts: SecureToolOptions{
			Context:     secCtx,
			Prompter:    &approval.FakePrompter{Decision: approval.DecisionApproveOnce},
			AuditSink:   audit.NewMemorySink(),
			RuleSet:     rules.NewSet(),
			RateLimiter: security.NewRateLimiter(),
		}.withDefaults(),
	}
	result, err := invokeSecureTool(
		context.Background(),
		factory,
		security.ToolDescriptor{Name: "create_file", Provider: security.ToolProviderBuiltin, Kind: security.ToolKindFileWrite, Risk: security.OperationRiskLow},
		security.OperationRequest{Operation: security.OperationWrite, TargetPath: "x.txt", Risk: security.OperationRiskLow},
		nil,
		WriteFileInput{Path: "x.txt", Content: "hello"},
		func(context.Context, WriteFileInput) (*FileMutationOutput, error) {
			t.Fatalf("handler should not run for suggested plan write")
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("invokeSecureTool() error = %v", err)
	}
	if result.Status != security.ResultStatusSuggested {
		t.Fatalf("status = %q, want suggested", result.Status)
	}
}

func TestSaveAndLoadLatestSessionMetadata(t *testing.T) {
	secCtx := security.Context{
		SessionID:     "session-1",
		DataDir:       t.TempDir(),
		SessionMode:   security.SessionModeAgent,
		SandboxMode:   security.SandboxModeWorkspaceWrite,
		ApprovalMode:  security.ApprovalModeInteractive,
		WorkspaceRoot: t.TempDir(),
	}
	if err := SaveSessionMetadata(secCtx); err != nil {
		t.Fatalf("SaveSessionMetadata() error = %v", err)
	}
	metadata, err := LoadLatestSessionMetadata(secCtx.DataDir)
	if err != nil {
		t.Fatalf("LoadLatestSessionMetadata() error = %v", err)
	}
	if metadata == nil || metadata.SessionID != secCtx.SessionID {
		t.Fatalf("metadata = %+v, want session %q", metadata, secCtx.SessionID)
	}
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}
