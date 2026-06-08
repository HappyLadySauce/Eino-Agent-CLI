package agents

import "testing"

func TestAgentRegistry(t *testing.T) {
	registry, err := NewAgentRegistry(nil)
	if err != nil {
		t.Fatalf("NewAgentRegistry() error = %v", err)
	}

	if definitions := registry.List(); len(definitions) != 0 {
		t.Fatalf("List() len = %d, want 0", len(definitions))
	}

	definition := AgentDefinition{
		Name:           "review-agent",
		Description:    "reviews implementation details",
		SystemPrompt:   "review carefully",
		PermissionMode: PermissionModeReadonly,
	}
	if err := registry.Register(definition); err != nil {
		t.Fatalf("Register(dynamic) error = %v", err)
	}
	if got, err := registry.Get("review-agent"); err != nil || got.Name != definition.Name {
		t.Fatalf("Get(review-agent) = %+v, %v; want registered definition", got, err)
	}
	if _, err := registry.Get("missing"); err == nil {
		t.Fatalf("Get(missing) error = nil, want non-nil")
	}
	if err := registry.Register(definition); err == nil {
		t.Fatalf("Register(duplicate) error = nil, want non-nil")
	}
	if err := registry.Register(AgentDefinition{
		Name:           "BadName",
		Description:    "bad",
		SystemPrompt:   "bad",
		PermissionMode: PermissionModeReadonly,
	}); err == nil {
		t.Fatalf("Register(invalid name) error = nil, want non-nil")
	}
	if err := registry.Register(AgentDefinition{
		Name:           "missing-prompt",
		Description:    "bad",
		PermissionMode: PermissionModeReadonly,
	}); err == nil {
		t.Fatalf("Register(missing prompt) error = nil, want non-nil")
	}
}
