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

	history := messages.NewMessages(runtime.MaxHistoryMessages())
	prompts := scanPrompts(ctx, os.Stdin)

	for {
		fmt.Print("User> ")
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
		if err := dispatchCommand(ctx, runtime, history, command); err != nil {
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

func dispatchCommand(ctx context.Context, runtime *AgentRuntime, history *messages.Messages, command AgentCommand) error {
	switch command.Kind {
	case AgentCommandChat:
		return runChatCommand(ctx, runtime, history, command.Prompt)
	case AgentCommandExit:
		return errAgentExit
	case AgentCommandList:
		printAgentDefinitions(runtime.Definitions())
		return nil
	case AgentCommandRun:
		return runSubAgentCommand(ctx, runtime, command.AgentName, command.Prompt)
	case AgentCommandParallel:
		return runParallelCommand(ctx, runtime, command.AgentName, command.Tasks)
	default:
		return fmt.Errorf("unsupported command kind %d", command.Kind)
	}
}

func runChatCommand(ctx context.Context, runtime *AgentRuntime, history *messages.Messages, prompt string) error {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil
	}

	if err := history.Add(schema.UserMessage(prompt)); err != nil {
		return fmt.Errorf("append user message: %w", err)
	}

	fmt.Print("Assistant> ")
	result, err := runtime.RunMain(ctx, history.Get(), os.Stdout)
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

func runSubAgentCommand(ctx context.Context, runtime *AgentRuntime, agentName, prompt string) error {
	fmt.Printf("Assistant[%s]> ", agentName)
	_, err := runtime.RunSync(ctx, agentName, prompt, os.Stdout)
	fmt.Println()
	return err
}

func runParallelCommand(ctx context.Context, runtime *AgentRuntime, agentName string, prompts []string) error {
	tasks := NewWorkerTasks(agentName, prompts)
	results, err := RunParallelWorkers(ctx, runtime, tasks, ParallelOptions{
		MaxConcurrency: runtime.MaxParallelWorkers(),
	})
	if err != nil {
		return err
	}

	fmt.Printf("Parallel[%s]> %d task(s)\n", agentName, len(results))
	for _, result := range results {
		status := "ok"
		if result.Err != nil {
			status = "error"
		}
		fmt.Printf("\n[%d] %s duration=%s\n", result.ID, status, result.Duration.Round(0))
		if result.Err != nil {
			fmt.Printf("Error: %v\n", result.Err)
			continue
		}
		fmt.Println(strings.TrimSpace(result.Content))
	}
	return nil
}

func printAgentDefinitions(definitions []AgentDefinition) {
	fmt.Println("Agents:")
	for _, definition := range definitions {
		parallel := "no"
		if definition.CanRunInParallel {
			parallel = "yes"
		}
		fmt.Printf("- %s [%s, parallel=%s]: %s\n", definition.Name, definition.PermissionMode, parallel, definition.Description)
	}
}
