package session

import (
	"path/filepath"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/tools/core"
)

func memoryPath(runtime core.Runtime, name string) string {
	return filepath.Join(memoryDir(runtime), core.SafeName(name)+".md")
}
