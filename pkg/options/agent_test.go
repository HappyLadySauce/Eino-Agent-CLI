package options

import "testing"

func TestAgentOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		options *AgentOptions
		wantErr bool
	}{
		{
			name:    "default",
			options: NewAgentOptions(),
		},
		{
			name:    "too low",
			options: &AgentOptions{MaxParallelWorkers: 0},
			wantErr: true,
		},
		{
			name:    "too high",
			options: &AgentOptions{MaxParallelWorkers: MaxParallelWorkersLimit + 1},
			wantErr: true,
		},
		{
			name:    "nil",
			options: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("Validate() error = nil, want non-nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}
