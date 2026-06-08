package agents

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/commands"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/messages"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/middlewares"
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

	modeState := commands.NewModeState()
	history := messages.NewMessages()
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

		command, err := commands.ParseAgentCommand(promptResult.text)
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

// dispatchCommand dispatches the command to the appropriate handler.
// dispatchCommand 分发命令到相应的处理函数。
func dispatchCommand(ctx context.Context, runtime *AgentRuntime, modeState *commands.ModeState, history *messages.Messages, command commands.AgentCommand) error {
	// 根据命令类型分发到相应的处理函数。
	switch command.Kind {
	case commands.AgentCommandChat:
		// 运行聊天命令。
		return runChatCommand(ctx, runtime, modeState.Current(), history, command.Prompt)
	case commands.AgentCommandExit:
		// 退出命令。
		return errAgentExit
	case commands.AgentCommandModeSwitch:
		// 切换模式命令。
		if err := modeState.Switch(command.Mode); err != nil {
			return err
		}
		fmt.Printf("Switched to %s mode.\n", modeState.Current())
		return nil
	case commands.AgentCommandSubAgent:
		// 运行子命令。
		return runChatCommand(ctx, runtime, modeState.Current(), history, buildSubAgentDelegationPrompt(command.Prompt))
	default:
		// 不支持的命令类型。
		return fmt.Errorf("unsupported command kind %d", command.Kind)
	}
}

// runChatCommand runs the chat command.
// runChatCommand 运行聊天命令。
func runChatCommand(ctx context.Context, runtime *AgentRuntime, mode commands.SessionMode, history *messages.Messages, prompt string) error {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil
	}

	if err := history.Add(schema.UserMessage(prompt)); err != nil {
		return fmt.Errorf("append user message: %w", err)
	}

	runCtx, stats := middlewares.NewStatsContext(ctx)
	result, err := runtime.RunMain(runCtx, mode, history.Get(), os.Stdout)
	if err != nil {
		return err
	}
	writeStatsLine(os.Stdout, stats, runtime.MaxContextTokens())

	if result.Content != "" {
		if err := history.Add(schema.AssistantMessage(result.Content, nil)); err != nil {
			return fmt.Errorf("append assistant message: %w", err)
		}
	}
	return nil
}

// buildSubAgentDelegationPrompt builds the prompt for subagent delegation.
// buildSubAgentDelegationPrompt 构建子命令委托提示词。
func buildSubAgentDelegationPrompt(task string) string {
	return "请把下面任务委托给合适的 subagent。你可以先 list_agents，必要时 create_agent，然后 run_subagent。subagent 必须使用全新上下文，最终由你汇总结果。\n\nTask:\n" + strings.TrimSpace(task)
}

func writeStatsLine(writer io.Writer, stats *middlewares.TokenStats, maxContextTokens int) {
	if writer == nil || stats == nil {
		return
	}
	snapshot := stats.Snapshot()
	contextUsagePercent := 0.0
	if maxContextTokens > 0 {
		contextUsagePercent = float64(snapshot.TotalTokens) / float64(maxContextTokens) * 100
	}
	fmt.Fprintf(
		writer,
		"Stats: elapsed=%s prompt↑=%d completion↓=%d total=%d(%.2f%%)\n",
		snapshot.Duration.Round(10_000_000).String(),
		snapshot.PromptTokens,
		snapshot.CompletionTokens,
		snapshot.TotalTokens,
		contextUsagePercent,
	)
}
