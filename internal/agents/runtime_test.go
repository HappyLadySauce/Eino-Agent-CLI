package agents

import (
	"strings"
	"testing"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/commands"
)

func TestNormalizeAgentName(t *testing.T) {
	if got, want := normalizeAgentName(" Error_Checker Agent "), "error-checker-agent"; got != want {
		t.Fatalf("normalizeAgentName() = %q, want %q", got, want)
	}
}

func TestValidateCreateAgentPermissionRequiresExplicitMode(t *testing.T) {
	_, err := validateCreateAgentPermission(commands.SessionModeAgent, "")
	if err == nil || !strings.Contains(err.Error(), "permission_mode is required") {
		t.Fatalf("validateCreateAgentPermission(agent, empty) error = %v, want required error", err)
	}
}

func TestValidateCreateAgentPermissionRejectsAskMode(t *testing.T) {
	_, err := validateCreateAgentPermission(commands.SessionModeAsk, string(PermissionModeReadonly))
	if err == nil || !strings.Contains(err.Error(), "ask mode cannot create sub-agents") {
		t.Fatalf("validateCreateAgentPermission(ask) error = %v, want ask mode error", err)
	}
}

func TestValidateCreateAgentPermissionLimitsPlanMode(t *testing.T) {
	_, err := validateCreateAgentPermission(commands.SessionModePlan, string(PermissionModeDefault))
	if err == nil || !strings.Contains(err.Error(), "plan mode cannot create agent") {
		t.Fatalf("validateCreateAgentPermission(plan, default) error = %v, want plan permission error", err)
	}

	permission, err := validateCreateAgentPermission(commands.SessionModePlan, string(PermissionModeReadonly))
	if err != nil {
		t.Fatalf("validateCreateAgentPermission(plan, readonly) error = %v", err)
	}
	if permission != PermissionModeReadonly {
		t.Fatalf("permission = %q, want %q", permission, PermissionModeReadonly)
	}
}

func TestValidateRunSubAgentRequestRequiresAgentName(t *testing.T) {
	err := validateRunSubAgentRequest(commands.SessionModeAgent, "")
	if err == nil || !strings.Contains(err.Error(), "agent_name is required") {
		t.Fatalf("validateRunSubAgentRequest(agent, empty) error = %v, want required error", err)
	}
}

func TestValidateRunSubAgentRequestRejectsAskMode(t *testing.T) {
	err := validateRunSubAgentRequest(commands.SessionModeAsk, "review-agent")
	if err == nil || !strings.Contains(err.Error(), "ask mode cannot run sub-agents") {
		t.Fatalf("validateRunSubAgentRequest(ask) error = %v, want ask mode error", err)
	}
}
