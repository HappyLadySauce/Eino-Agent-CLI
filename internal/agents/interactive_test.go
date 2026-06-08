package agents

import (
	"bytes"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/middlewares"
)

func TestWriteStatsLinePrintsCurrentTurnSummary(t *testing.T) {
	stats := &middlewares.TokenStats{}
	stats.RecordUsage(&schema.TokenUsage{PromptTokens: 3, CompletionTokens: 5, TotalTokens: 8})
	stats.RecordUsage(&schema.TokenUsage{PromptTokens: 7, CompletionTokens: 2, TotalTokens: 9})
	stats.RecordDuration(150 * time.Millisecond)

	var out bytes.Buffer
	writeStatsLine(&out, stats, 200, 21)

	got := out.String()
	want := "Stats: elapsed=150ms prompt↑=10 completion↓=7 turn=17 total=21 context=3.50%\n"
	if got != want {
		t.Fatalf("stats line = %q, want %q", got, want)
	}
}

func TestSessionTokenStatsAccumulatesTurns(t *testing.T) {
	session := &sessionTokenStats{}

	first := session.AddTurn(middlewares.TokenStatsSnapshot{TotalTokens: 8})
	second := session.AddTurn(middlewares.TokenStatsSnapshot{TotalTokens: 13})

	if first != 8 || second != 21 || session.Total() != 21 {
		t.Fatalf("unexpected session totals: first=%d second=%d total=%d", first, second, session.Total())
	}
}
