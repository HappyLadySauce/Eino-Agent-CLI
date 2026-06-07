package agents

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/messages"
)

type fakeAgentRunner struct {
	delay time.Duration

	mu        sync.Mutex
	active    int
	maxActive int
}

func (r *fakeAgentRunner) RunSync(ctx context.Context, agentName, prompt string, writer io.Writer) (*messages.AssistantStreamResult, error) {
	r.mu.Lock()
	r.active++
	if r.active > r.maxActive {
		r.maxActive = r.active
	}
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.active--
		r.mu.Unlock()
	}()

	select {
	case <-time.After(r.delay):
	case <-ctx.Done():
		return nil, fmt.Errorf("fake runner canceled: %w", ctx.Err())
	}

	if prompt == "fail" {
		return nil, fmt.Errorf("forced failure")
	}
	content := agentName + ":" + prompt
	if _, err := io.WriteString(writer, content); err != nil {
		return nil, err
	}
	return &messages.AssistantStreamResult{Content: content}, nil
}

func TestRunParallelWorkersKeepsOrderAndConcurrencyLimit(t *testing.T) {
	runner := &fakeAgentRunner{delay: 20 * time.Millisecond}
	tasks := NewWorkerTasks(AgentExplore, []string{"a", "b", "c"})

	results, err := RunParallelWorkers(context.Background(), runner, tasks, ParallelOptions{MaxConcurrency: 2})
	if err != nil {
		t.Fatalf("RunParallelWorkers() error = %v", err)
	}
	if runner.maxActive > 2 {
		t.Fatalf("max active = %d, want <= 2", runner.maxActive)
	}
	for i, result := range results {
		wantID := i + 1
		if result.ID != wantID {
			t.Fatalf("result ID = %d, want %d", result.ID, wantID)
		}
		if result.Err != nil {
			t.Fatalf("result %d error = %v", result.ID, result.Err)
		}
	}
	if results[0].Content != "explore:a" || results[2].Content != "explore:c" {
		t.Fatalf("results = %+v, want ordered content", results)
	}
}

func TestRunParallelWorkersKeepsPerTaskError(t *testing.T) {
	runner := &fakeAgentRunner{}
	tasks := NewWorkerTasks(AgentVerify, []string{"ok", "fail", "also-ok"})

	results, err := RunParallelWorkers(context.Background(), runner, tasks, ParallelOptions{MaxConcurrency: 3})
	if err != nil {
		t.Fatalf("RunParallelWorkers() error = %v", err)
	}
	if results[1].Err == nil {
		t.Fatalf("results[1].Err = nil, want non-nil")
	}
	if results[0].Err != nil || results[2].Err != nil {
		t.Fatalf("unexpected sibling errors: %+v", results)
	}
}
