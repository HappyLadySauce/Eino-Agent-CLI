package middlewares

import (
	"context"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	messageutils "github.com/HappyLadySauce/Eino-Agent-CLI/pkg/utils/messages"
	tokenutils "github.com/HappyLadySauce/Eino-Agent-CLI/pkg/utils/tokens"
)

type statsContextKey struct{}

// TokenMiddlewareConfig configures per-call message reduction and token accounting.
// TokenMiddlewareConfig 配置每次模型调用前的消息裁剪与 token 统计。
type TokenMiddlewareConfig struct {
	ModelName          string
	TokenizerModel     string
	MaxContextTokens   int
	MaxOutputTokens    int
	MaxHistoryMessages int
}

// TokenStatsSnapshot is an immutable view of accumulated model usage for one user turn.
// TokenStatsSnapshot 是单轮用户请求模型使用情况的不可变快照。
type TokenStatsSnapshot struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CallCount        int
	Duration         time.Duration
}

// TokenStats accumulates model usage for the current user turn.
// TokenStats 汇总当前用户请求中的模型使用情况。
type TokenStats struct {
	mu sync.RWMutex

	start            time.Time
	promptTokens     int
	completionTokens int
	totalTokens      int
	callCount        int
	duration         time.Duration
}

// NewStatsContext attaches a fresh token stats accumulator to ctx.
// NewStatsContext 在 ctx 上挂载新的 token 统计器。
func NewStatsContext(ctx context.Context) (context.Context, *TokenStats) {
	if ctx == nil {
		ctx = context.Background()
	}
	stats := &TokenStats{start: time.Now()}
	return context.WithValue(ctx, statsContextKey{}, stats), stats
}

// StatsFromContext returns the token stats accumulator attached to ctx.
// StatsFromContext 返回 ctx 上挂载的 token 统计器。
func StatsFromContext(ctx context.Context) *TokenStats {
	if ctx == nil {
		return nil
	}
	stats, _ := ctx.Value(statsContextKey{}).(*TokenStats)
	return stats
}

// RecordUsage records one model call and any token usage returned by the model.
// RecordUsage 记录一次模型调用，以及模型返回的 token 使用量。
func (s *TokenStats) RecordUsage(usage *schema.TokenUsage) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.callCount++
	if usage == nil {
		return
	}
	s.promptTokens += usage.PromptTokens
	s.completionTokens += usage.CompletionTokens
	s.totalTokens += usage.TotalTokens
}

// RecordDuration adds a measured model-call duration to the current stats.
// RecordDuration 累加模型调用耗时。
func (s *TokenStats) RecordDuration(duration time.Duration) {
	if s == nil || duration <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.duration += duration
}

// Snapshot returns a stable copy of current stats.
// Snapshot 返回当前统计的稳定副本。
func (s *TokenStats) Snapshot() TokenStatsSnapshot {
	if s == nil {
		return TokenStatsSnapshot{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	duration := s.duration
	if !s.start.IsZero() {
		duration = time.Since(s.start)
	}
	return TokenStatsSnapshot{
		PromptTokens:     s.promptTokens,
		CompletionTokens: s.completionTokens,
		TotalTokens:      s.totalTokens,
		CallCount:        s.callCount,
		Duration:         duration,
	}
}

type tokenMiddleware struct {
	*adk.BaseChatModelAgentMiddleware

	config  TokenMiddlewareConfig
	counter *tokenutils.TokenCounter
}

// NewTokenCountMiddleware creates a ChatModelAgent middleware for trimming and token accounting.
// NewTokenCountMiddleware 创建用于消息裁剪与 token 统计的 ChatModelAgent middleware。
func NewTokenCountMiddleware(config TokenMiddlewareConfig) (adk.ChatModelAgentMiddleware, error) {
	normalizeTokenConfig(&config)
	counter, err := tokenutils.NewTokenCounter(config.ModelName, config.TokenizerModel)
	if err != nil {
		return nil, err
	}
	return &tokenMiddleware{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
		config:                       config,
		counter:                      counter,
	}, nil
}

func normalizeTokenConfig(config *TokenMiddlewareConfig) {
	if config.MaxContextTokens <= 0 {
		config.MaxContextTokens = 128000
	}
	if config.MaxOutputTokens <= 0 {
		config.MaxOutputTokens = 32000
	}
	if config.MaxHistoryMessages <= 0 {
		config.MaxHistoryMessages = 40
	}
}

func (m *tokenMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if state == nil || len(state.Messages) == 0 {
		return ctx, state, nil
	}

	messages := messageutils.TrimByMessageCount(state.Messages, m.config.MaxHistoryMessages)
	budget := m.config.MaxContextTokens - m.config.MaxOutputTokens
	if budget > 0 {
		var err error
		messages, err = messageutils.TrimByTokenBudget(messages, state.ToolInfos, budget, m.counter)
		if err != nil {
			return ctx, state, err
		}
	}

	if messageutils.SameMessageSlice(messages, state.Messages) {
		return ctx, state, nil
	}
	trimmed := *state
	trimmed.Messages = messages
	return ctx, &trimmed, nil
}

func (m *tokenMiddleware) AfterModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if stats := StatsFromContext(ctx); stats != nil {
		stats.RecordUsage(messageutils.LatestAssistantUsage(state.Messages))
	}
	return ctx, state, nil
}

func (m *tokenMiddleware) WrapModel(ctx context.Context, next model.BaseModel[*schema.Message], mc *adk.ModelContext) (model.BaseModel[*schema.Message], error) {
	if next == nil {
		return next, nil
	}
	return &timedModel{next: next}, nil
}

type timedModel struct {
	next model.BaseModel[*schema.Message]
}

func (m *timedModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	start := time.Now()
	msg, err := m.next.Generate(ctx, input, opts...)
	if stats := StatsFromContext(ctx); stats != nil {
		stats.RecordDuration(time.Since(start))
	}
	return msg, err
}

func (m *timedModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	start := time.Now()
	stream, err := m.next.Stream(ctx, input, opts...)
	if stats := StatsFromContext(ctx); stats != nil {
		stats.RecordDuration(time.Since(start))
	}
	return stream, err
}
