package agents

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/messages"
)

// AgentRunner runs an agent with isolated output.
// AgentRunner 使用隔离输出运行 Agent。
type AgentRunner interface {
	RunSync(ctx context.Context, agentName, prompt string, writer io.Writer) (*messages.AssistantStreamResult, error)
}

// WorkerTask describes one parallel worker task.
// WorkerTask 描述一个并行 worker 任务。
type WorkerTask struct {
	ID        int
	AgentName string
	Prompt    string
}

// WorkerResult contains one parallel worker result.
// WorkerResult 包含一个并行 worker 结果。
type WorkerResult struct {
	ID        int
	AgentName string
	Prompt    string
	Content   string
	Err       error
	Duration  time.Duration
}

// ParallelOptions controls parallel worker execution.
// ParallelOptions 控制并行 worker 执行。
type ParallelOptions struct {
	MaxConcurrency int
}

// NewWorkerTasks builds stable worker tasks from prompts.
// NewWorkerTasks 根据 prompt 构建稳定顺序的 worker 任务。
func NewWorkerTasks(agentName string, prompts []string) []WorkerTask {
	tasks := make([]WorkerTask, 0, len(prompts))
	for i, prompt := range prompts {
		tasks = append(tasks, WorkerTask{
			ID:        i + 1,
			AgentName: agentName,
			Prompt:    prompt,
		})
	}
	return tasks
}

// RunParallelWorkers runs tasks with bounded concurrency and ordered results.
// RunParallelWorkers 使用有界并发运行任务并按输入顺序返回结果。
func RunParallelWorkers(ctx context.Context, runner AgentRunner, tasks []WorkerTask, opts ParallelOptions) ([]WorkerResult, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if runner == nil {
		return nil, errors.New("agent runner is required")
	}
	if len(tasks) == 0 {
		return nil, errors.New("at least one worker task is required")
	}
	if opts.MaxConcurrency < 1 {
		return nil, fmt.Errorf("max concurrency must be greater than or equal to 1, got %d", opts.MaxConcurrency)
	}

	results := make([]WorkerResult, len(tasks))
	sem := make(chan struct{}, opts.MaxConcurrency)
	var wg sync.WaitGroup

	for i, task := range tasks {
		if task.Prompt == "" {
			return nil, fmt.Errorf("worker task %d prompt is required", task.ID)
		}

		wg.Add(1)
		go func(index int, workerTask WorkerTask) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[index] = WorkerResult{
					ID:        workerTask.ID,
					AgentName: workerTask.AgentName,
					Prompt:    workerTask.Prompt,
					Err:       fmt.Errorf("worker canceled before start: %w", ctx.Err()),
				}
				return
			}

			start := time.Now()
			var out bytes.Buffer
			result, err := runner.RunSync(ctx, workerTask.AgentName, workerTask.Prompt, &out)
			content := out.String()
			if result != nil && content == "" {
				content = result.Content
			}
			results[index] = WorkerResult{
				ID:        workerTask.ID,
				AgentName: workerTask.AgentName,
				Prompt:    workerTask.Prompt,
				Content:   content,
				Err:       err,
				Duration:  time.Since(start),
			}
		}(i, task)
	}

	wg.Wait()
	return results, nil
}
