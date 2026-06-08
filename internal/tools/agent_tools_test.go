package tools

import (
	"context"
	"strings"
	"testing"
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
	if len(tools) != 3 {
		t.Fatalf("NewAgentTools() len = %d, want 3", len(tools))
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
	for _, name := range []string{"list_agents", "create_agent", "run_subagent"} {
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

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}
