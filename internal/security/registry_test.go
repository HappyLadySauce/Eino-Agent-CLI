package security

import "testing"

func TestRegistryRegisterAndExpose(t *testing.T) {
	registry := NewRegistry()
	descriptor := ToolDescriptor{
		Name:     "read_file",
		Provider: ToolProviderBuiltin,
		Kind:     ToolKindFileRead,
		Risk:     OperationRiskLow,
	}
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	got, err := registry.MustExpose(ToolProviderBuiltin, "read_file")
	if err != nil {
		t.Fatalf("MustExpose() error = %v", err)
	}
	if got.Name != descriptor.Name {
		t.Fatalf("descriptor name = %q, want %q", got.Name, descriptor.Name)
	}
}

func TestRegistryRejectsUnregisteredTool(t *testing.T) {
	registry := NewRegistry()
	_, err := registry.MustExpose(ToolProviderBuiltin, "run_command")
	if err == nil {
		t.Fatalf("MustExpose(unregistered) error = nil, want error")
	}
}

func TestRegistryRejectsDuplicateTool(t *testing.T) {
	registry := NewRegistry()
	descriptor := ToolDescriptor{
		Name:     "read_file",
		Provider: ToolProviderBuiltin,
		Kind:     ToolKindFileRead,
		Risk:     OperationRiskLow,
	}
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("Register(first) error = %v", err)
	}
	if err := registry.Register(descriptor); err == nil {
		t.Fatalf("Register(duplicate) error = nil, want error")
	}
}
