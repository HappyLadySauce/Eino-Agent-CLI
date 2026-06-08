package middlewares

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestTokenMiddlewareTrimsByMessageCountAndTokenBudget(t *testing.T) {
	handler, err := NewTokenCountMiddleware(TokenMiddlewareConfig{
		ModelName:          "unknown-local-model",
		MaxContextTokens:   80,
		MaxOutputTokens:    16,
		MaxHistoryMessages: 10,
	})
	if err != nil {
		t.Fatalf("NewTokenCountMiddleware() error = %v", err)
	}

	state := &adk.ChatModelAgentState{
		Messages: []*schema.Message{
			schema.SystemMessage("system can be removed " + strings.Repeat("s ", 200)),
			schema.UserMessage("old user " + strings.Repeat("x ", 200)),
			schema.AssistantMessage("old assistant "+strings.Repeat("y ", 200), nil),
			schema.UserMessage("latest user must remain"),
		},
	}

	_, trimmed, err := handler.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("BeforeModelRewriteState() error = %v", err)
	}

	contents := messageContents(trimmed.Messages)
	if !strings.Contains(contents, "latest user must remain") {
		t.Fatalf("trimmed messages missing latest user: %q", contents)
	}
	if strings.Contains(contents, "system can be removed") || strings.Contains(contents, "old user") || strings.Contains(contents, "old assistant") {
		t.Fatalf("trimmed messages still contain old history: %q", contents)
	}
}

func TestTokenMiddlewareKeepsMessagesWithinBudgetUnchanged(t *testing.T) {
	handler, err := NewTokenCountMiddleware(TokenMiddlewareConfig{
		ModelName:          "unknown-local-model",
		MaxContextTokens:   128000,
		MaxOutputTokens:    32000,
		MaxHistoryMessages: 10,
	})
	if err != nil {
		t.Fatalf("NewTokenCountMiddleware() error = %v", err)
	}

	original := []*schema.Message{
		schema.SystemMessage("system"),
		schema.UserMessage("first"),
		schema.AssistantMessage("second", nil),
		schema.UserMessage("latest"),
	}
	state := &adk.ChatModelAgentState{Messages: original}

	_, result, err := handler.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("BeforeModelRewriteState() error = %v", err)
	}
	if len(result.Messages) != len(original) {
		t.Fatalf("messages len = %d, want %d", len(result.Messages), len(original))
	}
	for i, msg := range result.Messages {
		if msg != original[i] {
			t.Fatalf("message %d changed: got %#v want %#v", i, msg, original[i])
		}
	}
}

func TestTokenStatsRecordsUsageAndCallCount(t *testing.T) {
	ctx, stats := NewStatsContext(context.Background())
	handler, err := NewTokenCountMiddleware(TokenMiddlewareConfig{
		ModelName:          "unknown-local-model",
		MaxContextTokens:   128000,
		MaxOutputTokens:    32000,
		MaxHistoryMessages: 10,
	})
	if err != nil {
		t.Fatalf("NewTokenCountMiddleware() error = %v", err)
	}

	withUsage := schema.AssistantMessage("ok", nil)
	withUsage.ResponseMeta = &schema.ResponseMeta{
		Usage: &schema.TokenUsage{PromptTokens: 11, CompletionTokens: 7, TotalTokens: 18},
	}
	_, _, err = handler.AfterModelRewriteState(ctx, &adk.ChatModelAgentState{
		Messages: []*schema.Message{schema.UserMessage("hello"), withUsage},
	}, nil)
	if err != nil {
		t.Fatalf("AfterModelRewriteState() with usage error = %v", err)
	}
	_, _, err = handler.AfterModelRewriteState(ctx, &adk.ChatModelAgentState{
		Messages: []*schema.Message{schema.UserMessage("hello"), schema.AssistantMessage("missing usage", nil)},
	}, nil)
	if err != nil {
		t.Fatalf("AfterModelRewriteState() without usage error = %v", err)
	}

	snapshot := stats.Snapshot()
	if snapshot.PromptTokens != 11 || snapshot.CompletionTokens != 7 || snapshot.TotalTokens != 18 {
		t.Fatalf("snapshot tokens = %#v, want prompt=11 completion=7 total=18", snapshot)
	}
	if snapshot.CallCount != 2 {
		t.Fatalf("CallCount = %d, want 2", snapshot.CallCount)
	}
}

func messageContents(messages []*schema.Message) string {
	var b strings.Builder
	for _, msg := range messages {
		if msg != nil {
			b.WriteString(msg.Content)
			b.WriteByte('\n')
		}
	}
	return b.String()
}
