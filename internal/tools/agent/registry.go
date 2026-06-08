package agent

import "github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"

var definitions []core.Definition

// Definitions returns all agent tool definitions registered by this package.
// Definitions 返回本包注册的全部 Agent 工具定义。
func Definitions() []core.Definition {
	return append([]core.Definition(nil), definitions...)
}

func register(definition core.Definition) {
	if definition == nil {
		panic("agent tool definition is nil")
	}
	definitions = append(definitions, definition)
}

func service(runtime core.Runtime) (Service, error) {
	svc, ok := runtime.Service.(Service)
	if !ok || svc == nil {
		return nil, ErrServiceMissing
	}
	return svc, nil
}
