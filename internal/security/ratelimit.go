package security

import (
	"sync"
	"time"
)

// RateLimiter tracks per-session and per-tool call counts.
// RateLimiter 跟踪每个会话和每个工具的调用次数。
type RateLimiter struct {
	mu       sync.Mutex
	now      func() time.Time
	sessions map[string]*sessionRateState
}

type sessionRateState struct {
	TotalCalls int
	Tools      map[string]*toolRateState
}

type toolRateState struct {
	TotalCalls   int
	WindowStart  time.Time
	WindowCalls  int
	DeniedStreak int
}

// NewRateLimiter creates an in-memory rate limiter.
// NewRateLimiter 创建内存速率限制器。
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		now:      time.Now,
		sessions: make(map[string]*sessionRateState),
	}
}

// CheckAndRecord records one call and returns a structured result if blocked.
// CheckAndRecord 记录一次调用，并在被限制时返回结构化结果。
func (l *RateLimiter) CheckAndRecord(sessionID, toolName string, limit RateLimitDescriptor) *ToolResult[struct{}] {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	session := l.sessions[sessionID]
	if session == nil {
		session = &sessionRateState{Tools: make(map[string]*toolRateState)}
		l.sessions[sessionID] = session
	}
	tool := session.Tools[toolName]
	if tool == nil {
		tool = &toolRateState{WindowStart: now}
		session.Tools[toolName] = tool
	}
	if now.Sub(tool.WindowStart) >= time.Minute {
		tool.WindowStart = now
		tool.WindowCalls = 0
	}
	if limit.MaxCallsPerSession > 0 && tool.TotalCalls >= limit.MaxCallsPerSession {
		return &ToolResult[struct{}]{
			OK:     false,
			Status: ResultStatusRateLimited,
			Reason: "tool session call limit exceeded",
		}
	}
	if limit.MaxCallsPerMinute > 0 && tool.WindowCalls >= limit.MaxCallsPerMinute {
		return &ToolResult[struct{}]{
			OK:     false,
			Status: ResultStatusRateLimited,
			Reason: "tool per-minute call limit exceeded",
		}
	}
	session.TotalCalls++
	tool.TotalCalls++
	tool.WindowCalls++
	return nil
}

// RecordDenied tracks repeated identical denials for backoff decisions.
// RecordDenied 跟踪重复拒绝以支持退避决策。
func (l *RateLimiter) RecordDenied(sessionID, toolName string) int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	session := l.sessions[sessionID]
	if session == nil {
		session = &sessionRateState{Tools: make(map[string]*toolRateState)}
		l.sessions[sessionID] = session
	}
	tool := session.Tools[toolName]
	if tool == nil {
		tool = &toolRateState{WindowStart: l.now()}
		session.Tools[toolName] = tool
	}
	tool.DeniedStreak++
	return tool.DeniedStreak
}
