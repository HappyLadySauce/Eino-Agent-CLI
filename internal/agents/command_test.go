package agents

import "testing"

func TestParseAgentCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    AgentCommand
		wantErr bool
	}{
		{
			name:  "chat",
			input: "你好",
			want:  AgentCommand{Kind: AgentCommandChat, Prompt: "你好"},
		},
		{
			name:  "exit",
			input: "exit",
			want:  AgentCommand{Kind: AgentCommandExit},
		},
		{
			name:  "list",
			input: "/agents",
			want:  AgentCommand{Kind: AgentCommandList},
		},
		{
			name:  "plan",
			input: "/plan 做一个方案",
			want:  AgentCommand{Kind: AgentCommandRun, AgentName: AgentPlan, Prompt: "做一个方案"},
		},
		{
			name:  "agent",
			input: "/agent explore 分析代码",
			want:  AgentCommand{Kind: AgentCommandRun, AgentName: AgentExplore, Prompt: "分析代码"},
		},
		{
			name:  "parallel",
			input: "/parallel verify 检查错误 || 检查并发",
			want:  AgentCommand{Kind: AgentCommandParallel, AgentName: AgentVerify, Tasks: []string{"检查错误", "检查并发"}},
		},
		{
			name:    "unknown",
			input:   "/bad command",
			wantErr: true,
		},
		{
			name:    "empty plan",
			input:   "/plan ",
			wantErr: true,
		},
		{
			name:    "empty parallel task",
			input:   "/parallel explore ||",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAgentCommand(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseAgentCommand() error = nil, want non-nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAgentCommand() error = %v", err)
			}
			if got.Kind != tt.want.Kind || got.AgentName != tt.want.AgentName || got.Prompt != tt.want.Prompt {
				t.Fatalf("ParseAgentCommand() = %+v, want %+v", got, tt.want)
			}
			if len(got.Tasks) != len(tt.want.Tasks) {
				t.Fatalf("ParseAgentCommand() tasks = %+v, want %+v", got.Tasks, tt.want.Tasks)
			}
			for i := range got.Tasks {
				if got.Tasks[i] != tt.want.Tasks[i] {
					t.Fatalf("ParseAgentCommand() tasks = %+v, want %+v", got.Tasks, tt.want.Tasks)
				}
			}
		})
	}
}
