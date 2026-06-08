package agents

import (
	"context"
	"fmt"
)

type promptResult struct {
	text string
	err  error
}

// receivePrompt waits for either the next prompt or context cancellation.
// receivePrompt 等待下一条用户输入或 context 取消。
func receivePrompt(ctx context.Context, prompts <-chan promptResult) (promptResult, bool) {
	select {
	case <-ctx.Done():
		return promptResult{err: fmt.Errorf("agent loop canceled: %w", ctx.Err())}, true
	case prompt, ok := <-prompts:
		return prompt, ok
	}
}
