package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	einotool "github.com/cloudwego/eino/components/tool"

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
	tools, err := NewSecureAgentTools("plan", fakeAgentToolService{}, SecureToolOptions{
		Context:     secCtx,
		Prompter:    &approval.FakePrompter{Decision: approval.DecisionApproveOnce},
		AuditSink:   audit.NewMemorySink(),
		RuleSet:     rules.NewSet(),
		RateLimiter: security.NewRateLimiter(),
	})
	if err != nil {
		t.Fatalf("NewSecureAgentTools() error = %v", err)
	}
	createFile := findInvokableTool(t, tools, "create_file")
	raw, err := createFile.InvokableRun(context.Background(), `{"path":"x.txt","content":"hello"}`)
	if err != nil {
		t.Fatalf("create_file InvokableRun() error = %v", err)
	}
	var result security.ToolResult[json.RawMessage]
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal tool result: %v; raw=%s", err, raw)
	}
	if result.Status != security.ResultStatusSuggested {
		t.Fatalf("status = %q, want suggested; raw=%s", result.Status, raw)
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

func findInvokableTool(t *testing.T, tools []einotool.BaseTool, name string) einotool.InvokableTool {
	t.Helper()
	for _, candidate := range tools {
		info, err := candidate.Info(context.Background())
		if err != nil {
			t.Fatalf("Info() error = %v", err)
		}
		if info.Name != name {
			continue
		}
		invokable, ok := candidate.(einotool.InvokableTool)
		if !ok {
			t.Fatalf("tool %q is not invokable", name)
		}
		return invokable
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}
