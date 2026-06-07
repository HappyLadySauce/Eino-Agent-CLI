package agents

import (
	"bufio"
	"context"
	"fmt"
	"io"
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

	// Use ChatModelAgent directly for CLI chat; LoopAgent re-invokes sub-agents until
	// MaxIterations or BreakLoop, and MaxIterations=0 means infinite API calls.
	// CLI 对话直接使用 ChatModelAgent；LoopAgent 会反复调用子 Agent，
	// 且 MaxIterations=0 表示无限循环调用 API。
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agentChatModel,
		EnableStreaming: true,
	})

	var messages []*schema.Message
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("User> ")
		if !scanner.Scan() {
			break
		}
		prompt := strings.TrimSpace(scanner.Text())
		if prompt == "quit" {
			break
		}

		messages = append(messages, schema.UserMessage(prompt))
		iter := runner.Run(ctx, messages)
		fmt.Print("Assistant> ")

		var reply strings.Builder
		for {
			event, ok := iter.Next()
			if !ok {
				break
			}
			if event.Err != nil {
				return event.Err
			}
			if event.Output == nil || event.Output.MessageOutput == nil {
				continue
			}
			mo := event.Output.MessageOutput
			if mo.Role != schema.Assistant {
				continue
			}

			if mo.IsStreaming && mo.MessageStream != nil {
				if err := streamAssistantReply(mo.MessageStream, &reply); err != nil {
					return err
				}
				continue
			}

			msg, _, err := adk.GetMessage(event)
			if err != nil {
				return err
			}
			if msg.Content == "" {
				continue
			}

			fmt.Print(msg.Content)
			reply.WriteString(msg.Content)
		}
		fmt.Println()

		if reply.Len() > 0 {
			messages = append(messages, schema.AssistantMessage(reply.String(), nil))
		}
	}
	
	return nil
}

// streamAssistantReply prints assistant tokens as they arrive and accumulates the full reply.
// streamAssistantReply 流式打印 assistant 输出，并累积完整回复用于多轮上下文。
func streamAssistantReply(stream *schema.StreamReader[*schema.Message], reply *strings.Builder) error {
	defer stream.Close()
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if chunk == nil || chunk.Content == "" {
			continue
		}
		fmt.Print(chunk.Content)
		reply.WriteString(chunk.Content)
	}
}