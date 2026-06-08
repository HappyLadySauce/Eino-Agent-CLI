package security

import "testing"

func TestValidateExternalToolDescriptorRejectsUnknownAutoApprove(t *testing.T) {
	err := ValidateExternalToolDescriptor(ToolDescriptor{
		Name:        "plugin_upload",
		Provider:    ToolProviderPlugin,
		Kind:        ToolKindExternalState,
		Risk:        OperationRiskMedium,
		AutoApprove: true,
		Resources:   []ResourceDescriptor{{Kind: "domain", Unknown: true}},
	})
	if err == nil {
		t.Fatalf("ValidateExternalToolDescriptor() error = nil, want error")
	}
}

func TestEvaluateNetworkPolicyRejectsPrivateByDefault(t *testing.T) {
	decision := EvaluateNetworkPolicy(Context{SessionMode: SessionModeAgent}, NetworkPolicy{}, []string{"http://127.0.0.1:8080"})
	if decision.Decision != DecisionDeny {
		t.Fatalf("decision = %q, want deny", decision.Decision)
	}
}

func TestEvaluateNetworkPolicyUnknownTargetByMode(t *testing.T) {
	if got := EvaluateNetworkPolicy(Context{SessionMode: SessionModePlan}, NetworkPolicy{}, nil); got.Decision != DecisionSuggest {
		t.Fatalf("plan unknown decision = %q, want suggest", got.Decision)
	}
	if got := EvaluateNetworkPolicy(Context{SessionMode: SessionModeAsk}, NetworkPolicy{}, nil); got.Decision != DecisionDeny {
		t.Fatalf("ask unknown decision = %q, want deny", got.Decision)
	}
}
