package file

import "github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"

var definitions []core.Definition

// Definitions returns all file tool definitions registered by this package.
// Definitions 返回本包注册的全部文件工具定义。
func Definitions() []core.Definition {
	return append([]core.Definition(nil), definitions...)
}

func register(definition core.Definition) {
	if definition == nil {
		panic("file tool definition is nil")
	}
	definitions = append(definitions, definition)
}
