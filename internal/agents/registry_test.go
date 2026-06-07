package agents

import "testing"

func TestAgentRegistry(t *testing.T) {
	registry, err := NewAgentRegistry(BuiltinAgentDefinitions())
	if err != nil {
		t.Fatalf("NewAgentRegistry() error = %v", err)
	}

	definitions := registry.List()
	if len(definitions) != 4 {
		t.Fatalf("List() len = %d, want 4", len(definitions))
	}

	if _, err := registry.Get(AgentPlan); err != nil {
		t.Fatalf("Get(%q) error = %v", AgentPlan, err)
	}
	if _, err := registry.Get("missing"); err == nil {
		t.Fatalf("Get(missing) error = nil, want non-nil")
	}
	if err := registry.Register(definitions[0]); err == nil {
		t.Fatalf("Register(duplicate) error = nil, want non-nil")
	}
}
