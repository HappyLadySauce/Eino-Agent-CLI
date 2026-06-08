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
	stats.RecordDuration(150 * time.Millisecond)

	var out bytes.Buffer
	writeStatsLine(&out, stats, 200)

	got := out.String()
	want := "Stats: elapsed=150ms prompt=3tokens completion=5tokens total=8tokens(4.00%) calls=1\n"
	if got != want {
		t.Fatalf("stats line = %q, want %q", got, want)
	}
}
