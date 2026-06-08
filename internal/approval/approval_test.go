package approval

import (
	"context"
	"strings"
	"testing"
)

func TestCLIPrompterApproveOnce(t *testing.T) {
	var out strings.Builder
	prompter := NewCLIPrompter(strings.NewReader("y\n"), &out)
	decision, err := prompter.Prompt(context.Background(), Request{ToolName: "run_command", Reason: "test"})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if decision != DecisionApproveOnce {
		t.Fatalf("decision = %q, want approve_once", decision)
	}
	if !strings.Contains(out.String(), "Tool: run_command") {
		t.Fatalf("prompt output = %q, want tool name", out.String())
	}
}

func TestFakePrompterDefaultsDeny(t *testing.T) {
	decision, err := (&FakePrompter{}).Prompt(context.Background(), Request{})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if decision != DecisionDeny {
		t.Fatalf("decision = %q, want deny", decision)
	}
}
