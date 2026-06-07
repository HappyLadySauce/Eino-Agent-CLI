package agents

import (
	"fmt"
	"sort"
)

// AgentRegistry stores available agent definitions by name.
// AgentRegistry 按名称保存可用 Agent 定义。
type AgentRegistry struct {
	definitions map[string]AgentDefinition
}

// NewAgentRegistry creates a registry from definitions.
// NewAgentRegistry 根据定义创建注册表。
func NewAgentRegistry(definitions []AgentDefinition) (*AgentRegistry, error) {
	registry := &AgentRegistry{
		definitions: make(map[string]AgentDefinition, len(definitions)),
	}
	for _, definition := range definitions {
		if err := registry.Register(definition); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

// Register adds one agent definition.
// Register 添加一个 Agent 定义。
func (r *AgentRegistry) Register(definition AgentDefinition) error {
	if r == nil {
		return fmt.Errorf("agent registry is nil")
	}
	if definition.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if definition.Description == "" {
		return fmt.Errorf("agent %q description is required", definition.Name)
	}
	if _, exists := r.definitions[definition.Name]; exists {
		return fmt.Errorf("agent %q is already registered", definition.Name)
	}
	r.definitions[definition.Name] = definition
	return nil
}

// Get returns one agent definition by name.
// Get 按名称返回一个 Agent 定义。
func (r *AgentRegistry) Get(name string) (AgentDefinition, error) {
	if r == nil {
		return AgentDefinition{}, fmt.Errorf("agent registry is nil")
	}
	definition, ok := r.definitions[name]
	if !ok {
		return AgentDefinition{}, fmt.Errorf("agent %q is not registered", name)
	}
	return definition, nil
}

// List returns all definitions in stable name order.
// List 按稳定名称顺序返回所有定义。
func (r *AgentRegistry) List() []AgentDefinition {
	if r == nil || len(r.definitions) == 0 {
		return nil
	}
	definitions := make([]AgentDefinition, 0, len(r.definitions))
	for _, definition := range r.definitions {
		definitions = append(definitions, definition)
	}
	sort.Slice(definitions, func(i, j int) bool {
		return definitions[i].Name < definitions[j].Name
	})
	return definitions
}
