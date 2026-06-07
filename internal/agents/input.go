package agents

import (
	"bufio"
	"context"
	"fmt"
	"io"
)

type promptResult struct {
	text string
	err  error
}

// scanPrompts reads input on a worker goroutine so the main loop can react to context cancellation.
// scanPrompts 在工作 goroutine 中读取输入，使主循环可以响应 context 取消。
func scanPrompts(ctx context.Context, reader io.Reader) <-chan promptResult {
	prompts := make(chan promptResult)
	go func() {
		defer close(prompts)

		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case prompts <- promptResult{text: scanner.Text()}:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case prompts <- promptResult{err: err}:
			case <-ctx.Done():
			}
		}
	}()
	return prompts
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
