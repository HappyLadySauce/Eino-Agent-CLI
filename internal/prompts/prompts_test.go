package prompts

import (
	"strings"
	"testing"
)

func TestMainAgentInstructionByMode(t *testing.T) {
	if got := MainAgentInstruction(ModePlan); !strings.Contains(got, "Plan 模式") {
		t.Fatalf("MainAgentInstruction(plan) = %q, want Plan mode prompt", got)
	} else if !strings.Contains(got, "permission_mode") {
		t.Fatalf("MainAgentInstruction(plan) = %q, want explicit permission_mode guidance", got)
	}
	if got := MainAgentInstruction(ModeAsk); !strings.Contains(got, "Ask 模式") {
		t.Fatalf("MainAgentInstruction(ask) = %q, want Ask mode prompt", got)
	}
	if got := MainAgentInstruction(ModeAgents); !strings.Contains(got, "Agents 模式") {
		t.Fatalf("MainAgentInstruction(agents) = %q, want Agents mode prompt", got)
	} else if !strings.Contains(got, "permission_mode") {
		t.Fatalf("MainAgentInstruction(agents) = %q, want explicit permission_mode guidance", got)
	}
}

func TestSubAgentPrompt(t *testing.T) {
	got := SubAgentPrompt("task", "ctx", "json")
	for _, want := range []string{"Task:\ntask", "Context summary from main agent:\nctx", "Expected output:\njson"} {
		if !strings.Contains(got, want) {
			t.Fatalf("SubAgentPrompt() = %q, want substring %q", got, want)
		}
	}
}
