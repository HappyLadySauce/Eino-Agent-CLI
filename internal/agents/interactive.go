package agents

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/messages"
	"github.com/HappyLadySauce/Eino-Agent-CLI/pkg/config"
)

func RunAgentLoop(ctx context.Context, cfg *config.Config) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil || cfg.Model == nil {
		return errors.New("agent config is missing model settings")
	}

	model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  cfg.Model.AuthToken,
		BaseURL: cfg.Model.BaseURL,
		Model:   cfg.Model.Model,
	})
	if err != nil {
		return fmt.Errorf("create chat model: %w", err)
	}

	chatAgent, err := NewChatAgent(ctx, model)
	if err != nil {
		return fmt.Errorf("create chat agent: %w", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           chatAgent.agentChatModel,
		EnableStreaming: true,
	})

	msgs := messages.NewMessages(cfg.Model.MaxOutputTokens)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer
	prompts := scanPrompts(ctx, scanner)

	for {
		fmt.Print("User> ")
		promptResult, ok := receivePrompt(ctx, prompts)
		if !ok {
			return nil
		}
		if promptResult.err != nil {
			return fmt.Errorf("read user input: %w", promptResult.err)
		}

		prompt := strings.TrimSpace(promptResult.text)
		if prompt == "" {
			continue
		}
		if prompt == "quit" || prompt == "exit" {
			break
		}

		msgs.Add(schema.UserMessage(prompt))
		iter := runner.Run(ctx, msgs.Get())
		fmt.Print("Assistant> ")

		result, err := messages.ConsumeAssistantStream(iter, os.Stdout)
		if err != nil {
			return fmt.Errorf("consume assistant stream: %w", err)
		}
		fmt.Println()

		if result.Content != "" {
			msgs.Add(schema.AssistantMessage(result.Content, nil))
		}
	}

	return nil
}

type promptResult struct {
	text string
	err  error
}

// scanPrompts reads stdin on a worker goroutine so the main loop can react to context cancellation.
// scanPrompts 在工作 goroutine 中读取 stdin，使主循环可以响应 context 取消。
func scanPrompts(ctx context.Context, scanner *bufio.Scanner) <-chan promptResult {
	prompts := make(chan promptResult)
	go func() {
		defer close(prompts)

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
