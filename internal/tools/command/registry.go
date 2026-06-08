package command

import "github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"

var definitions []core.Definition

// Definitions returns all command tool definitions registered by this package.
// Definitions 返回本包注册的全部命令工具定义。
func Definitions() []core.Definition {
	return append([]core.Definition(nil), definitions...)
}

func register(definition core.Definition) {
	if definition == nil {
		panic("command tool definition is nil")
	}
	definitions = append(definitions, definition)
}
