package security

import (
	"fmt"
	"net"
)

// ValidateExternalToolDescriptor validates MCP/plugin descriptor requirements.
// ValidateExternalToolDescriptor 校验 MCP/plugin 工具描述符要求。
func ValidateExternalToolDescriptor(descriptor ToolDescriptor) error {
	if err := ValidateToolDescriptor(descriptor); err != nil {
		return err
	}
	if descriptor.Provider != ToolProviderMCP && descriptor.Provider != ToolProviderPlugin {
		return fmt.Errorf("external descriptor provider must be mcp or plugin")
	}
	if len(descriptor.Resources) == 0 {
		return fmt.Errorf("external descriptor must declare resources")
	}
	for _, resource := range descriptor.Resources {
		if resource.Unknown && descriptor.AutoApprove {
			return fmt.Errorf("unknown external resources cannot be auto-approved")
		}
	}
	return nil
}

// EvaluateNetworkPolicy evaluates declared network targets.
// EvaluateNetworkPolicy 评估声明的网络目标。
func EvaluateNetworkPolicy(ctx Context, policy NetworkPolicy, targets []string) PolicyDecision {
	if len(targets) == 0 {
		switch ctx.SessionMode {
		case SessionModeAsk:
			return PolicyDecision{Decision: DecisionDeny, Reason: "unknown network target is denied in ask mode"}
		case SessionModePlan:
			return PolicyDecision{Decision: DecisionSuggest, Reason: "unknown network target is suggested in plan mode"}
		default:
			return PolicyDecision{Decision: DecisionAsk, Reason: "unknown network target requires approval"}
		}
	}
	for _, target := range targets {
		host := NormalizeHostname(target)
		if host == "" {
			return PolicyDecision{Decision: DecisionDeny, Reason: "network target host is empty"}
		}
		if IsPrivateOrLocalHost(host) && !policy.AllowPrivateIPs {
			return PolicyDecision{Decision: DecisionDeny, Reason: "private or local network target is denied"}
		}
		for _, denied := range policy.DeniedDomains {
			if host == NormalizeHostname(denied) {
				return PolicyDecision{Decision: DecisionDeny, Reason: "network target is denied by policy"}
			}
		}
		if len(policy.AllowedDomains) > 0 {
			allowed := false
			for _, domain := range policy.AllowedDomains {
				if host == NormalizeHostname(domain) {
					allowed = true
					break
				}
			}
			if !allowed {
				return PolicyDecision{Decision: DecisionAsk, Reason: "network target is not allowlisted"}
			}
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
			return PolicyDecision{Decision: DecisionDeny, Reason: "unspecified network target is denied"}
		}
	}
	if policy.DefaultDecision != "" {
		return PolicyDecision{Decision: policy.DefaultDecision, Reason: "network target matched policy"}
	}
	return PolicyDecision{Decision: DecisionAsk, Reason: "network target requires approval"}
}
