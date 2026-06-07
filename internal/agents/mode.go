package agents

import "fmt"

// ModeState stores the current interactive session mode.
// ModeState 保存当前交互会话模式。
type ModeState struct {
	current SessionMode
}

// NewModeState creates a mode state using Agent mode by default.
// NewModeState 创建默认使用 Agent 模式的模式状态。
func NewModeState() *ModeState {
	return &ModeState{current: SessionModeAgents}
}

// Current returns the active session mode.
// Current 返回当前会话模式。
func (s *ModeState) Current() SessionMode {
	if s == nil || s.current == "" {
		return SessionModeAgents
	}
	return s.current
}

// Switch changes the active session mode.
// Switch 切换当前会话模式。
func (s *ModeState) Switch(mode SessionMode) error {
	if s == nil {
		return fmt.Errorf("mode state is nil")
	}
	switch mode {
	case SessionModeAgents, SessionModePlan, SessionModeAsk:
		s.current = mode
		return nil
	default:
		return fmt.Errorf("unsupported session mode %q", mode)
	}
}
