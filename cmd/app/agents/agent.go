package agents

import (
	"context"
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/HappyLadySauce/Eino-Agent-CLI/cmd/app/options"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"k8s.io/klog/v2"
)

func RunAgentLoop(ctx context.Context, opts *options.Options) error {

	model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey: opts.Model.AuthToken,
		BaseURL: opts.Model.BaseURL,
		Model: opts.Model.Model,
	})
	if err != nil {
		klog.Errorf("failed to create chat model: %v", err)
		return err
	}

	agentChatModel, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name: "agent-chat-model",
		Description: "this is a chat model agent that can answer questions",
		Model: model,
	})
	if err != nil {
		klog.Errorf("failed to create chat model agent: %v", err)
		return err
	}

	agentLoop, err := adk.NewLoopAgent(ctx, &adk.LoopAgentConfig{
		Name: "agent-loop",
		Description: "this is a loop agent",
		SubAgents: []adk.Agent{agentChatModel},
	})
	if err != nil {
		klog.Errorf("failed to create loop agent: %v", err)
		return err
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent: agentLoop,
	})

	var messages []*schema.Message
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		prompt := strings.TrimSpace(scanner.Text())
		if prompt == "quit" {
			break
		}

		messages = append(messages, schema.UserMessage(prompt))
		iter := runner.Run(ctx, messages)
		for {
			event, ok := iter.Next()
			if !ok {
				break
			}
			if event.Err != nil {
				klog.Errorf("failed to run agent: %v", event.Err)
				continue
			}
			if event.Output != nil && event.Output.MessageOutput != nil {
				fmt.Println(event.Output.MessageOutput.Message.Content)
			}
		}
	}
	
	return nil
}