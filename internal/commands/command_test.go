package commands

import (
	"testing"
)

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
			name:  "agent mode",
			input: "/agent",
			want:  AgentCommand{Kind: AgentCommandModeSwitch, Mode: SessionModeAgent},
		},
		{
			name:  "plan mode",
			input: "/plan",
			want:  AgentCommand{Kind: AgentCommandModeSwitch, Mode: SessionModePlan},
		},
		{
			name:  "ask mode",
			input: "/ask",
			want:  AgentCommand{Kind: AgentCommandModeSwitch, Mode: SessionModeAsk},
		},
		{
			name:  "subagent",
			input: "/subagent 检查错误传递",
			want:  AgentCommand{Kind: AgentCommandSubAgent, Prompt: "检查错误传递"},
		},
		{
			name:    "old agent command removed",
			input:   "/agent explore 分析代码",
			wantErr: true,
		},
		{
			name:    "old parallel command removed",
			input:   "/parallel verify 检查错误 || 检查并发",
			wantErr: true,
		},
		{
			name:    "empty subagent",
			input:   "/subagent ",
			wantErr: true,
		},
		{
			name:    "unknown",
			input:   "/bad command",
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
			if got != tt.want {
				t.Fatalf("ParseAgentCommand() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestModeState(t *testing.T) {
	state := NewModeState()
	if got := state.Current(); got != SessionModeAgent {
		t.Fatalf("Current() = %q, want %q", got, SessionModeAgent)
	}
	if err := state.Switch(SessionModePlan); err != nil {
		t.Fatalf("Switch(plan) error = %v", err)
	}
	if got := state.Current(); got != SessionModePlan {
		t.Fatalf("Current() = %q, want %q", got, SessionModePlan)
	}
	if err := state.Switch("bad"); err == nil {
		t.Fatalf("Switch(bad) error = nil, want non-nil")
	}
}
