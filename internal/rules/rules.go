package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.starlark.net/starlark"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
)

// Source identifies rule ownership for precedence.
// Source 标识规则来源，用于优先级排序。
type Source string

const (
	SourceProject Source = "project"
	SourceUser    Source = "user"
)

// Rule is the internal Go rule AST built from Starlark declarations.
// Rule 是由 Starlark 声明构建的 Go 内部规则 AST。
type Rule struct {
	Source      Source
	Kind        string
	Pattern     []string
	Tool        string
	SessionMode security.SessionMode
	Operation   security.OperationKind
	Decision    security.Decision
}

// Set stores validated rules and reload state.
// Set 存储已验证规则和重载状态。
type Set struct {
	mu          sync.RWMutex
	rules       []Rule
	reloadError error
}

// NewSet creates an empty rule set.
// NewSet 创建空规则集。
func NewSet() *Set {
	return &Set{}
}

// LoadFiles parses existing rule files. Missing files are ignored.
// LoadFiles 解析已存在的规则文件，缺失文件会被忽略。
func LoadFiles(projectPath, userPath string) (*Set, error) {
	set := NewSet()
	var all []Rule
	for _, item := range []struct {
		path   string
		source Source
	}{
		{path: projectPath, source: SourceProject},
		{path: userPath, source: SourceUser},
	} {
		if strings.TrimSpace(item.path) == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Clean(item.path))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read rules file %s: %w", item.path, err)
		}
		parsed, err := Parse(string(data), item.source)
		if err != nil {
			return nil, fmt.Errorf("parse rules file %s: %w", item.path, err)
		}
		all = append(all, parsed...)
	}
	set.rules = all
	return set, nil
}

// Reload replaces rules only when parsing succeeds.
// Reload 仅在解析成功时替换规则。
func (s *Set) Reload(projectText, userText string) error {
	if s == nil {
		return fmt.Errorf("rules set is nil")
	}
	var all []Rule
	for _, item := range []struct {
		text   string
		source Source
	}{
		{projectText, SourceProject},
		{userText, SourceUser},
	} {
		if strings.TrimSpace(item.text) == "" {
			continue
		}
		parsed, err := Parse(item.text, item.source)
		if err != nil {
			s.mu.Lock()
			s.reloadError = err
			s.mu.Unlock()
			return err
		}
		all = append(all, parsed...)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules = all
	s.reloadError = nil
	return nil
}

// Evaluate applies explicit rules to one request.
// Evaluate 将显式规则应用到一个请求。
func (s *Set) Evaluate(ctx security.Context, req security.OperationRequest, tokens []string) (security.PolicyDecision, bool) {
	if s == nil {
		return security.PolicyDecision{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.reloadError != nil && (req.Risk == security.OperationRiskHigh || req.Risk == security.OperationRiskDestructive || req.Unknown) {
		return security.PolicyDecision{Decision: security.DecisionDeny, Reason: "rules reload error denies high-risk operation"}, true
	}
	for _, decision := range []security.Decision{security.DecisionDeny, security.DecisionAsk, security.DecisionAllow} {
		for _, source := range []Source{SourceProject, SourceUser} {
			for _, rule := range s.rules {
				if rule.Source == source && rule.Decision == decision && rule.matches(ctx, req, tokens) {
					return security.PolicyDecision{Decision: decision, Reason: "matched " + rule.Kind + " rule"}, true
				}
			}
		}
	}
	return security.PolicyDecision{}, false
}

// Parse parses Starlark rule declarations.
// Parse 解析 Starlark 规则声明。
func Parse(text string, source Source) ([]Rule, error) {
	collector := &ruleCollector{source: source}
	thread := &starlark.Thread{Name: "rules"}
	globals := starlark.StringDict{
		"prefix_rule": starlark.NewBuiltin("prefix_rule", collector.prefixRule),
		"glob_rule":   starlark.NewBuiltin("glob_rule", collector.globRule),
		"tool_rule":   starlark.NewBuiltin("tool_rule", collector.toolRule),
		"when":        starlark.NewBuiltin("when", collector.whenRule),
	}
	if _, err := starlark.ExecFile(thread, "rules.star", text, globals); err != nil {
		return nil, err
	}
	return collector.rules, nil
}

type ruleCollector struct {
	source Source
	rules  []Rule
}

func (c *ruleCollector) prefixRule(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	rule := Rule{Source: c.source, Kind: "prefix"}
	var pattern *starlark.List
	var operation, decision string
	if err := starlark.UnpackArgs("prefix_rule", args, kwargs, "pattern", &pattern, "operation", &operation, "decision", &decision); err != nil {
		return nil, err
	}
	for i := 0; i < pattern.Len(); i++ {
		value := pattern.Index(i)
		text, ok := starlark.AsString(value)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("prefix_rule pattern must contain non-empty strings")
		}
		rule.Pattern = append(rule.Pattern, text)
	}
	if err := finalizeRule(&rule, operation, decision); err != nil {
		return nil, err
	}
	c.rules = append(c.rules, rule)
	return starlark.None, nil
}

func (c *ruleCollector) globRule(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, operation, decision string
	rule := Rule{Source: c.source, Kind: "glob"}
	if err := starlark.UnpackArgs("glob_rule", args, kwargs, "pattern", &pattern, "operation", &operation, "decision", &decision); err != nil {
		return nil, err
	}
	rule.Pattern = []string{pattern}
	if err := finalizeRule(&rule, operation, decision); err != nil {
		return nil, err
	}
	c.rules = append(c.rules, rule)
	return starlark.None, nil
}

func (c *ruleCollector) toolRule(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var tool, operation, decision string
	rule := Rule{Source: c.source, Kind: "tool"}
	if err := starlark.UnpackArgs("tool_rule", args, kwargs, "tool", &tool, "operation", &operation, "decision", &decision); err != nil {
		return nil, err
	}
	rule.Tool = tool
	if err := finalizeRule(&rule, operation, decision); err != nil {
		return nil, err
	}
	c.rules = append(c.rules, rule)
	return starlark.None, nil
}

func (c *ruleCollector) whenRule(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var sessionMode, tool, operation, decision string
	rule := Rule{Source: c.source, Kind: "when"}
	if err := starlark.UnpackArgs("when", args, kwargs, "session_mode", &sessionMode, "tool", &tool, "operation", &operation, "decision", &decision); err != nil {
		return nil, err
	}
	rule.SessionMode = security.SessionMode(sessionMode)
	rule.Tool = tool
	if err := finalizeRule(&rule, operation, decision); err != nil {
		return nil, err
	}
	c.rules = append(c.rules, rule)
	return starlark.None, nil
}

func finalizeRule(rule *Rule, operation, decision string) error {
	rule.Operation = security.OperationKind(operation)
	if !isKnownOperation(rule.Operation) {
		return fmt.Errorf("unsupported operation %q", operation)
	}
	rule.Decision = security.Decision(decision)
	switch rule.Decision {
	case security.DecisionAllow, security.DecisionAsk, security.DecisionDeny:
		return nil
	default:
		return fmt.Errorf("unsupported decision %q", decision)
	}
}

func isKnownOperation(operation security.OperationKind) bool {
	switch operation {
	case security.OperationRead, security.OperationList, security.OperationWrite, security.OperationDelete,
		security.OperationExec, security.OperationNetwork, security.OperationUpload,
		security.OperationMemoryWrite, security.OperationExternalState:
		return true
	default:
		return false
	}
}

func (r Rule) matches(ctx security.Context, req security.OperationRequest, tokens []string) bool {
	if r.Operation != req.Operation {
		return false
	}
	if r.SessionMode != "" && r.SessionMode != ctx.SessionMode {
		return false
	}
	switch r.Kind {
	case "prefix":
		if len(tokens) < len(r.Pattern) {
			return false
		}
		for i := range r.Pattern {
			if !strings.EqualFold(tokens[i], r.Pattern[i]) {
				return false
			}
		}
		return true
	case "glob":
		if req.TargetPath == "" || len(r.Pattern) != 1 {
			return false
		}
		ok, _ := filepath.Match(r.Pattern[0], filepath.Base(req.TargetPath))
		return ok
	case "tool", "when":
		return r.Tool == "" || r.Tool == req.Tool.Name
	default:
		return false
	}
}
