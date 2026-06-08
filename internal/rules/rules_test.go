package rules

import (
	"strings"
	"testing"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
)

func TestParseStarlarkRules(t *testing.T) {
	parsed, err := Parse(`
prefix_rule(pattern = ["git", "status"], operation = "exec", decision = "allow")
glob_rule(pattern = "*.env", operation = "read", decision = "deny")
tool_rule(tool = "delete_file", operation = "delete", decision = "ask")
when(session_mode = "plan", tool = "replace_file", operation = "write", decision = "deny")
`, SourceProject)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(parsed) != 4 {
		t.Fatalf("len(parsed) = %d, want 4", len(parsed))
	}
}

func TestParseRequiresOperationKind(t *testing.T) {
	_, err := Parse(`glob_rule(pattern = "*.env", decision = "deny")`, SourceProject)
	if err == nil || !strings.Contains(err.Error(), "missing argument") {
		t.Fatalf("Parse() error = %v, want missing operation", err)
	}
}

func TestEvaluatePrefixUsesTokens(t *testing.T) {
	set := NewSet()
	if err := set.Reload(`prefix_rule(pattern = ["git", "status"], operation = "exec", decision = "allow")`, ""); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	ctx := security.Context{SessionMode: security.SessionModeAgent}
	req := security.OperationRequest{Operation: security.OperationExec, Tool: security.ToolDescriptor{Name: "run_command"}}
	decision, ok := set.Evaluate(ctx, req, []string{"git", "status"})
	if !ok || decision.Decision != security.DecisionAllow {
		t.Fatalf("Evaluate() = %+v, %v, want allow", decision, ok)
	}
	_, ok = set.Evaluate(ctx, req, []string{"git", "status; rm -rf ."})
	if ok {
		t.Fatalf("Evaluate(chained token) matched unexpectedly")
	}
}

func TestReloadKeepsLastValidAndDeniesHighRiskOnError(t *testing.T) {
	set := NewSet()
	if err := set.Reload(`tool_rule(tool = "run_command", operation = "exec", decision = "allow")`, ""); err != nil {
		t.Fatalf("Reload(valid) error = %v", err)
	}
	if err := set.Reload(`tool_rule(`, ""); err == nil {
		t.Fatalf("Reload(invalid) error = nil, want error")
	}
	req := security.OperationRequest{
		Operation: security.OperationExec,
		Risk:      security.OperationRiskHigh,
		Tool:      security.ToolDescriptor{Name: "run_command"},
	}
	decision, ok := set.Evaluate(security.Context{}, req, []string{"go", "get"})
	if !ok || decision.Decision != security.DecisionDeny {
		t.Fatalf("Evaluate(high risk after reload error) = %+v, %v, want deny", decision, ok)
	}
}
