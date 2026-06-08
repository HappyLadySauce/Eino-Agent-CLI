package agents

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/cloudwego/eino/schema"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/approval"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/commands"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/messages"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/middlewares"
	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/terminal"
	"github.com/HappyLadySauce/Eino-Agent-CLI/pkg/config"
)

var errAgentExit = errors.New("agent exit requested")

// sessionTokenStats tracks cumulative token usage for one interactive session.
// sessionTokenStats 跟踪单个交互式会话的累计 token 使用量。
type sessionTokenStats struct {
	mu          sync.RWMutex
	totalTokens int
}

// AddTurn adds one completed user turn to the cumulative session usage.
// Parameters:
//   - turn: immutable token usage snapshot for the completed turn.
//
// Returns:
//   - int: cumulative total tokens after adding this turn.
//
// Example:
//
//	total := session.AddTurn(stats.Snapshot())
//
// AddTurn 将一次完成的用户请求加入会话累计用量。
// 参数：
//   - turn：已完成请求的不可变 token 用量快照。
//
// 返回值：
//   - int：加入本轮后的累计 total tokens。
//
// 使用示例：
//
//	total := session.AddTurn(stats.Snapshot())
func (s *sessionTokenStats) AddTurn(turn middlewares.ContextStatsSnapshot) int {
	if s == nil || turn.TotalTokens <= 0 {
		return s.Total()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.totalTokens += turn.TotalTokens
	return s.totalTokens
}

// Total returns the cumulative token usage for the session.
// Total 返回当前会话累计 token 使用量。
func (s *sessionTokenStats) Total() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalTokens
}

// RunAgentLoop starts the interactive agent CLI loop.
// RunAgentLoop 启动交互式 Agent CLI 循环。
func RunAgentLoop(ctx context.Context, cfg *config.Config) error {
	if ctx == nil {
		return errors.New("context is required")
	}

	inputRouter := NewInputRouter(ctx, os.Stdin)
	runtime, err := NewAgentRuntime(ctx, cfg, approval.NewCLIPrompter(inputRouter, os.Stdout))
	if err != nil {
		return fmt.Errorf("create agent runtime: %w", err)
	}

	modeState := commands.NewModeState()
	history := messages.NewMessages()
	sessionStats := &sessionTokenStats{}
	prompts := inputRouter.ChatPrompts()
	stdoutStyle := terminal.StyleForWriter(os.Stdout)

	for {
		fmt.Print(stdoutStyle.UserPrompt(fmt.Sprintf("User[%s]> ", modeState.Current())))
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
		if err := dispatchCommand(ctx, runtime, modeState, history, sessionStats, command); err != nil {
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
func dispatchCommand(ctx context.Context, runtime *AgentRuntime, modeState *commands.ModeState, history *messages.Messages, sessionStats *sessionTokenStats, command commands.AgentCommand) error {
	// 根据命令类型分发到相应的处理函数。
	switch command.Kind {
	case commands.AgentCommandChat:
		// 运行聊天命令。
		return runChatCommand(ctx, runtime, modeState.Current(), history, sessionStats, command.Prompt)
	case commands.AgentCommandExit:
		// 退出命令。
		return errAgentExit
	case commands.AgentCommandModeSwitch:
		// 切换模式命令。
		if err := modeState.Switch(command.Mode); err != nil {
			return err
		}
		stdoutStyle := terminal.StyleForWriter(os.Stdout)
		fmt.Print(stdoutStyle.UserPrompt(fmt.Sprintf("Switched to %s mode.\n", modeState.Current())))
		return nil
	default:
		// 不支持的命令类型。
		return fmt.Errorf("unsupported command kind %d", command.Kind)
	}
}

// runChatCommand runs the chat command.
// runChatCommand 运行聊天命令。
func runChatCommand(ctx context.Context, runtime *AgentRuntime, mode commands.SessionMode, history *messages.Messages, sessionStats *sessionTokenStats, prompt string) error {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil
	}

	if err := history.Add(schema.UserMessage(prompt)); err != nil {
		return fmt.Errorf("append user message: %w", err)
	}

	runCtx, stats := middlewares.NewStatsContext(ctx)
	assistantOut := terminal.NewAnimatedWriter(os.Stdout, "Thinking")
	result, err := runtime.RunMain(runCtx, mode, history.Get(), assistantOut)
	if closeErr := assistantOut.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	sessionTotal := sessionStats.AddTurn(stats.Snapshot())
	writeStatsLine(os.Stdout, stats, runtime.MaxContextTokens(), sessionTotal)

	if result.Content != "" {
		if err := history.Add(schema.AssistantMessage(result.Content, nil)); err != nil {
			return fmt.Errorf("append assistant message: %w", err)
		}
	}
	return nil
}

func writeStatsLine(writer io.Writer, stats *middlewares.ContextStats, maxContextTokens int, sessionTotalTokens int) {
	if writer == nil || stats == nil {
		return
	}
	snapshot := stats.Snapshot()
	displayTotalTokens := sessionTotalTokens
	if displayTotalTokens <= 0 {
		displayTotalTokens = snapshot.TotalTokens
	}
	contextUsagePercent := 0.0
	if maxContextTokens > 0 {
		contextPromptTokens := snapshot.MaxPromptTokens
		if contextPromptTokens <= 0 {
			contextPromptTokens = snapshot.PromptTokens
		}
		contextUsagePercent = float64(contextPromptTokens) / float64(maxContextTokens) * 100
	}
	fmt.Fprintf(
		writer,
		terminal.StyleForWriter(writer).Stats("Stats: elapsed=%s prompt↑=%d completion↓=%d turn=%d total=%d context=%.2f%%\n"),
		snapshot.Duration.Round(10_000_000).String(),
		snapshot.PromptTokens,
		snapshot.CompletionTokens,
		snapshot.TotalTokens,
		displayTotalTokens,
		contextUsagePercent,
	)
}
