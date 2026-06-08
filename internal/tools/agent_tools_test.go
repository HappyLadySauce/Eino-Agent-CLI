package tools

import (
	"context"
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
	for _, tool := range tools {
		info, err := tool.Info(context.Background())
		if err != nil {
			t.Fatalf("Info() error = %v", err)
		}
		names[info.Name] = true
	}
	for _, name := range []string{"list_agents", "create_agent", "run_subagent"} {
		if !names[name] {
			t.Fatalf("NewAgentTools() missing tool %q", name)
		}
	}
}
