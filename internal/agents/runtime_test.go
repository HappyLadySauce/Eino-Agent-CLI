package agents

import "testing"

func TestNormalizeAgentName(t *testing.T) {
	if got, want := normalizeAgentName(" Error_Checker Agent "), "error-checker-agent"; got != want {
		t.Fatalf("normalizeAgentName() = %q, want %q", got, want)
	}
}
