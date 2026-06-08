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

// ContextMiddlewareConfig configures per-call message trimming against the model context window.
// ContextMiddlewareConfig 配置每次模型调用前的消息裁剪与上下文窗口预算。
type ContextMiddlewareConfig struct {
	ModelName          string // 模型名称
	TokenizerModel     string // 分词器模型名称
	MaxContextTokens   int    // 最大上下文窗口大小
	MaxOutputTokens    int    // 最大输出窗口大小
	MaxHistoryMessages int    // 最大历史消息数量
}

// ContextStatsSnapshot is an immutable view of accumulated context window usage for one user turn.
// ContextStatsSnapshot 是单轮用户请求模型上下文窗口使用情况的不可变快照。
type ContextStatsSnapshot struct {
	PromptTokens     int           // 模型输入提示词 token 数量
	MaxPromptTokens  int           // 最大模型输入提示词 token 数量
	CompletionTokens int           // 模型输出完成词 token 数量
	TotalTokens      int           // 总 token 数量
	CallCount        int           // 调用次数
	Duration         time.Duration // 模型调用耗时
}

// ContextStats accumulates context window usage for the current user turn.
// ContextStats 汇总当前用户请求中的上下文窗口使用情况。
type ContextStats struct {
	mu sync.RWMutex

	start            time.Time
	promptTokens     int           // 模型输入提示词 token 数量
	maxPromptTokens  int           // 最大提示词 token 数量
	completionTokens int           // 模型输出完成词 token 数量
	totalTokens      int           // 总 token 数量
	callCount        int           // 调用次数
	duration         time.Duration // 模型调用耗时
}

// NewStatsContext attaches a fresh context stats accumulator to ctx.
// NewStatsContext 在 ctx 上挂载新的上下文窗口统计器。
func NewStatsContext(ctx context.Context) (context.Context, *ContextStats) {
	if ctx == nil {
		ctx = context.Background()
	}
	stats := &ContextStats{start: time.Now()}
	return context.WithValue(ctx, statsContextKey{}, stats), stats
}

// StatsFromContext returns the context stats accumulator attached to ctx.
// StatsFromContext 返回 ctx 上挂载的上下文窗口统计器。
func StatsFromContext(ctx context.Context) *ContextStats {
	if ctx == nil {
		return nil
	}
	stats, _ := ctx.Value(statsContextKey{}).(*ContextStats)
	return stats
}

// RecordUsage records one model call and any context window usage returned by the model.
// RecordUsage 记录一次模型调用，以及模型返回的上下文窗口使用量。
func (s *ContextStats) RecordUsage(usage *schema.TokenUsage) {
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
	if usage.PromptTokens > s.maxPromptTokens {
		s.maxPromptTokens = usage.PromptTokens
	}
	s.completionTokens += usage.CompletionTokens
	s.totalTokens += usage.TotalTokens
}

// RecordDuration adds a measured model-call duration to the current stats.
// RecordDuration 累加模型调用耗时。
func (s *ContextStats) RecordDuration(duration time.Duration) {
	if s == nil || duration <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.duration += duration
}

// Snapshot returns a stable copy of current stats.
// Snapshot 返回当前统计的稳定副本。
func (s *ContextStats) Snapshot() ContextStatsSnapshot {
	if s == nil {
		return ContextStatsSnapshot{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	duration := s.duration
	if !s.start.IsZero() {
		duration = time.Since(s.start)
	}
	return ContextStatsSnapshot{
		PromptTokens:     s.promptTokens,
		MaxPromptTokens:  s.maxPromptTokens,
		CompletionTokens: s.completionTokens,
		TotalTokens:      s.totalTokens,
		CallCount:        s.callCount,
		Duration:         duration,
	}
}

type contextMiddleware struct {
	*adk.BaseChatModelAgentMiddleware

	config  ContextMiddlewareConfig
	counter *tokenutils.TokenCounter
}

// NewContextMiddleware creates a ChatModelAgent middleware for context-window trimming and usage accounting.
// NewContextMiddleware 创建用于上下文窗口裁剪与用量统计的 ChatModelAgent middleware。
func NewContextMiddleware(config ContextMiddlewareConfig) (adk.ChatModelAgentMiddleware, error) {
	normalizeContextConfig(&config)
	counter, err := tokenutils.NewTokenCounter(config.ModelName, config.TokenizerModel)
	if err != nil {
		return nil, err
	}
	return &contextMiddleware{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
		config:                       config,
		counter:                      counter,
	}, nil
}

func normalizeContextConfig(config *ContextMiddlewareConfig) {
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

func (m *contextMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if state == nil || len(state.Messages) == 0 {
		return ctx, state, nil
	}

	messages := messageutils.TrimByMessageCount(state.Messages, m.config.MaxHistoryMessages)
	budget := promptBudget(m.config.MaxContextTokens, m.config.MaxOutputTokens)
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

// promptBudget reserves output and estimation safety margin from the context window.
// promptBudget 从上下文窗口中扣除输出预算和估算安全余量。
func promptBudget(maxContextTokens, maxOutputTokens int) int {
	budget := maxContextTokens - maxOutputTokens
	if budget <= 0 {
		return budget
	}
	return budget - promptBudgetSafetyMargin(budget)
}

// promptBudgetSafetyMargin returns a bounded margin for tokenizer/provider drift.
// promptBudgetSafetyMargin 返回用于覆盖 tokenizer/provider 误差的有界安全余量。
func promptBudgetSafetyMargin(budget int) int {
	if budget <= 0 {
		return 0
	}
	margin := budget / 20
	if margin < 128 {
		return margin
	}
	if margin > 2048 {
		return 2048
	}
	return margin
}

func (m *contextMiddleware) AfterModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if stats := StatsFromContext(ctx); stats != nil {
		stats.RecordUsage(messageutils.LatestAssistantUsage(state.Messages))
	}
	return ctx, state, nil
}

func (m *contextMiddleware) WrapModel(ctx context.Context, next model.BaseModel[*schema.Message], mc *adk.ModelContext) (model.BaseModel[*schema.Message], error) {
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
