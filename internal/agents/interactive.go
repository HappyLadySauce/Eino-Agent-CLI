package agents

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/messages"
	"github.com/HappyLadySauce/Eino-Agent-CLI/pkg/config"
)

var errAgentExit = errors.New("agent exit requested")

// RunAgentLoop starts the interactive agent CLI loop.
// RunAgentLoop 启动交互式 Agent CLI 循环。
func RunAgentLoop(ctx context.Context, cfg *config.Config) error {
	if ctx == nil {
		return errors.New("context is required")
	}

	runtime, err := NewAgentRuntime(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create agent runtime: %w", err)
	}

	modeState := NewModeState()
	history := messages.NewMessages(runtime.MaxHistoryMessages())
	prompts := scanPrompts(ctx, os.Stdin)

	for {
		fmt.Printf("User[%s]> ", modeState.Current())
		promptResult, ok := receivePrompt(ctx, prompts)
		if !ok {
			return nil
		}
		if promptResult.err != nil {
			if errors.Is(promptResult.err, context.Canceled) || errors.Is(promptResult.err, context.DeadlineExceeded) {
				fmt.Fprintln(os.Stderr, "Agent loop stopped by context cancellation.")
				return nil
			}
			return fmt.Errorf("read user input: %w", promptResult.err)
		}

		command, err := ParseAgentCommand(promptResult.text)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Command error: %v\n", err)
			continue
		}
		if err := dispatchCommand(ctx, runtime, modeState, history, command); err != nil {
			if errors.Is(err, errAgentExit) {
				return nil
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				fmt.Fprintln(os.Stderr, "Agent command stopped by context cancellation.")
				return nil
			}
			fmt.Fprintf(os.Stderr, "Agent error: %v\n", err)
		}
	}
}

func dispatchCommand(ctx context.Context, runtime *AgentRuntime, modeState *ModeState, history *messages.Messages, command AgentCommand) error {
	switch command.Kind {
	case AgentCommandChat:
		return runChatCommand(ctx, runtime, modeState.Current(), history, command.Prompt)
	case AgentCommandExit:
		return errAgentExit
	case AgentCommandModeSwitch:
		if err := modeState.Switch(command.Mode); err != nil {
			return err
		}
		fmt.Printf("Switched to %s mode.\n", modeState.Current())
		return nil
	case AgentCommandSubAgent:
		return runChatCommand(ctx, runtime, modeState.Current(), history, buildSubAgentDelegationPrompt(command.Prompt))
	default:
		return fmt.Errorf("unsupported command kind %d", command.Kind)
	}
}

func runChatCommand(ctx context.Context, runtime *AgentRuntime, mode SessionMode, history *messages.Messages, prompt string) error {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil
	}

	if err := history.Add(schema.UserMessage(prompt)); err != nil {
		return fmt.Errorf("append user message: %w", err)
	}

	fmt.Print("Assistant> ")
	result, err := runtime.RunMain(ctx, mode, history.Get(), os.Stdout)
	if err != nil {
		return err
	}
	fmt.Println()

	if result.Content != "" {
		if err := history.Add(schema.AssistantMessage(result.Content, nil)); err != nil {
			return fmt.Errorf("append assistant message: %w", err)
		}
	}
	return nil
}

func buildSubAgentDelegationPrompt(task string) string {
	return "请把下面任务委托给合适的 subagent。你可以先 list_agents，必要时 create_agent，然后 run_subagent。subagent 必须使用全新上下文，最终由你汇总结果。\n\nTask:\n" + strings.TrimSpace(task)
}
