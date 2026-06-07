package agents

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"k8s.io/klog/v2"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/messages"
	"github.com/HappyLadySauce/Eino-Agent-CLI/pkg/config"
)

func RunAgentLoop(ctx context.Context, cfg *config.Config) error {

	model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  cfg.Model.AuthToken,
		BaseURL: cfg.Model.BaseURL,
		Model:   cfg.Model.Model,
	})
	if err != nil {
		klog.Errorf("failed to create chat model: %v", err)
		return err
	}

	chatAgent, err := NewChatAgent(ctx, model)
	if err != nil {
		klog.Errorf("failed to create chat agent: %v", err)
		return err
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           chatAgent.agentChatModel,
		EnableStreaming: true,
	})

	msgs := messages.NewMessages(cfg.Model.MaxOutputTokens)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	for {
		fmt.Print("User> ")
		if !scanner.Scan() {
			break
		}
		prompt := strings.TrimSpace(scanner.Text())
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
			return err
		}
		fmt.Println()

		if result.Content != "" {
			msgs.Add(schema.AssistantMessage(result.Content, nil))
		}
	}

	return nil
}
