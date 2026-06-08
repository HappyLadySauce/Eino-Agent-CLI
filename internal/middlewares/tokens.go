package middlewares

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	tiktoken "github.com/pkoukk/tiktoken-go"
)

const fallbackEncoding = "cl100k_base"

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

// TokenCounter estimates message and tool prompt tokens with a tiktoken encoding.
// TokenCounter 使用 tiktoken encoding 估算消息和工具 prompt token。
type TokenCounter struct {
	encoding *tiktoken.Tiktoken
}

// NewTokenCounter creates a tokenizer-backed token counter.
// NewTokenCounter 创建基于 tokenizer 的 token 计数器。
func NewTokenCounter(modelName, tokenizerModel string) (*TokenCounter, error) {
	name := strings.TrimSpace(tokenizerModel)
	if name == "" {
		name = strings.TrimSpace(modelName)
	}

	var encoding *tiktoken.Tiktoken
	var err error
	if name != "" {
		encoding, err = tiktoken.EncodingForModel(name)
	}
	if encoding == nil || err != nil {
		encoding, err = tiktoken.GetEncoding(fallbackEncoding)
	}
	if err != nil {
		return nil, err
	}
	return &TokenCounter{encoding: encoding}, nil
}

// CountMessages estimates prompt tokens for messages and tool definitions.
// CountMessages 估算消息和工具定义的 prompt token。
func (c *TokenCounter) CountMessages(messages []*schema.Message, tools []*schema.ToolInfo) (int, error) {
	if c == nil || c.encoding == nil {
		return 0, errors.New("token counter is not initialized")
	}

	total := 0
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		total += 4
		total += c.countText(string(msg.Role))
		total += c.countText(msg.Name)
		total += c.countText(msg.Content)
		total += c.countText(msg.ReasoningContent)
		total += c.countText(msg.ToolCallID)
		total += c.countText(msg.ToolName)
		for _, part := range msg.MultiContent {
			total += c.countJSON(part)
		}
		for _, part := range msg.UserInputMultiContent {
			total += c.countJSON(part)
		}
		for _, part := range msg.AssistantGenMultiContent {
			total += c.countJSON(part)
		}
		for _, call := range msg.ToolCalls {
			total += c.countJSON(call)
		}
	}
	for _, tool := range tools {
		total += c.countJSON(tool)
	}
	return total, nil
}

func (c *TokenCounter) countText(text string) int {
	if text == "" {
		return 0
	}
	return len(c.encoding.Encode(text, nil, nil))
}

func (c *TokenCounter) countJSON(value any) int {
	data, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	return c.countText(string(data))
}

type tokenMiddleware struct {
	*adk.BaseChatModelAgentMiddleware

	config  TokenMiddlewareConfig
	counter *TokenCounter
}

// NewTokenCountMiddleware creates a ChatModelAgent middleware for trimming and token accounting.
// NewTokenCountMiddleware 创建用于消息裁剪与 token 统计的 ChatModelAgent middleware。
func NewTokenCountMiddleware(config TokenMiddlewareConfig) (adk.ChatModelAgentMiddleware, error) {
	normalizeTokenConfig(&config)
	counter, err := NewTokenCounter(config.ModelName, config.TokenizerModel)
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

	messages := trimByMessageCount(state.Messages, m.config.MaxHistoryMessages)
	budget := m.config.MaxContextTokens - m.config.MaxOutputTokens
	if budget > 0 {
		var err error
		messages, err = trimByTokenBudget(messages, state.ToolInfos, budget, m.counter)
		if err != nil {
			return ctx, state, err
		}
	}

	if sameMessageSlice(messages, state.Messages) {
		return ctx, state, nil
	}
	trimmed := *state
	trimmed.Messages = messages
	return ctx, &trimmed, nil
}

func (m *tokenMiddleware) AfterModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if stats := StatsFromContext(ctx); stats != nil {
		stats.RecordUsage(latestAssistantUsage(state.Messages))
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

func trimByMessageCount(messages []*schema.Message, max int) []*schema.Message {
	if max <= 0 || len(messages) <= max {
		return messages
	}

	systemMessages := make([]*schema.Message, 0)
	nonSystem := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		if msg != nil && msg.Role == schema.System {
			systemMessages = append(systemMessages, msg)
			continue
		}
		nonSystem = append(nonSystem, msg)
	}
	if len(nonSystem) > max {
		nonSystem = nonSystem[len(nonSystem)-max:]
	}
	trimmed := make([]*schema.Message, 0, len(systemMessages)+len(nonSystem))
	trimmed = append(trimmed, systemMessages...)
	trimmed = append(trimmed, nonSystem...)
	return trimmed
}

func trimByTokenBudget(messages []*schema.Message, tools []*schema.ToolInfo, budget int, counter *TokenCounter) ([]*schema.Message, error) {
	trimmed := append([]*schema.Message(nil), messages...)
	for {
		total, err := counter.CountMessages(trimmed, tools)
		if err != nil {
			return messages, err
		}
		if total <= budget {
			return trimmed, nil
		}

		index := firstRemovableMessageIndex(trimmed)
		if index < 0 {
			return trimmed, nil
		}
		trimmed = append(trimmed[:index], trimmed[index+1:]...)
	}
}

func firstRemovableMessageIndex(messages []*schema.Message) int {
	latestUser := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i] != nil && messages[i].Role == schema.User {
			latestUser = i
			break
		}
	}
	for i, msg := range messages {
		if msg == nil {
			return i
		}
		if msg.Role == schema.System {
			continue
		}
		if i == latestUser {
			continue
		}
		return i
	}
	return -1
}

func latestAssistantUsage(messages []*schema.Message) *schema.TokenUsage {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg == nil || msg.Role != schema.Assistant {
			continue
		}
		if msg.ResponseMeta == nil {
			return nil
		}
		return msg.ResponseMeta.Usage
	}
	return nil
}

func sameMessageSlice(left, right []*schema.Message) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
