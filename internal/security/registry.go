package security

import (
	"fmt"
	"sync"
)

// Registry stores descriptors for tools that may be exposed to the model.
// Registry 存储允许暴露给模型的工具描述符。
type Registry struct {
	mu          sync.RWMutex
	descriptors map[string]ToolDescriptor
}

// NewRegistry creates an empty secure tool registry.
// NewRegistry 创建空的安全工具注册表。
func NewRegistry() *Registry {
	return &Registry{descriptors: make(map[string]ToolDescriptor)}
}

// Register validates and stores one tool descriptor.
// Register 校验并存储一个工具描述符。
func (r *Registry) Register(descriptor ToolDescriptor) error {
	if r == nil {
		return fmt.Errorf("security registry is nil")
	}
	if err := ValidateToolDescriptor(descriptor); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := descriptorKey(descriptor.Provider, descriptor.Name)
	if _, exists := r.descriptors[key]; exists {
		return fmt.Errorf("tool %q from provider %q is already registered", descriptor.Name, descriptor.Provider)
	}
	r.descriptors[key] = descriptor
	return nil
}

// Descriptor returns a registered descriptor.
// Descriptor 返回已注册的工具描述符。
func (r *Registry) Descriptor(provider ToolProvider, name string) (ToolDescriptor, bool) {
	if r == nil {
		return ToolDescriptor{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	descriptor, ok := r.descriptors[descriptorKey(provider, name)]
	return descriptor, ok
}

// MustExpose returns an error when a tool has not been registered.
// MustExpose 在工具未注册时返回错误。
func (r *Registry) MustExpose(provider ToolProvider, name string) (ToolDescriptor, error) {
	descriptor, ok := r.Descriptor(provider, name)
	if !ok {
		return ToolDescriptor{}, fmt.Errorf("tool %q from provider %q is not registered", name, provider)
	}
	return descriptor, nil
}

func descriptorKey(provider ToolProvider, name string) string {
	return string(provider) + ":" + name
}
