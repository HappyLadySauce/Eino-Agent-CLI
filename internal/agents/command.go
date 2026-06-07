package agents

import (
	"errors"
	"fmt"
	"strings"
)

// AgentCommandKind identifies one CLI agent command.
// AgentCommandKind 标识一个 CLI Agent 命令。
type AgentCommandKind int

const (
	AgentCommandChat AgentCommandKind = iota
	AgentCommandExit
	AgentCommandList
	AgentCommandRun
	AgentCommandParallel
)

// AgentCommand is the parsed representation of one user input line.
// AgentCommand 是用户输入行的解析结果。
type AgentCommand struct {
	Kind      AgentCommandKind
	AgentName string
	Prompt    string
	Tasks     []string
}

// ParseAgentCommand parses one input line into a command.
// ParseAgentCommand 将一行输入解析为命令。
func ParseAgentCommand(input string) (AgentCommand, error) {
	text := strings.TrimSpace(input)
	if text == "" {
		return AgentCommand{Kind: AgentCommandChat}, nil
	}
	switch text {
	case "exit", "quit":
		return AgentCommand{Kind: AgentCommandExit}, nil
	case "/agents":
		return AgentCommand{Kind: AgentCommandList}, nil
	}

	if strings.HasPrefix(text, "/plan ") {
		prompt := strings.TrimSpace(strings.TrimPrefix(text, "/plan "))
		if prompt == "" {
			return AgentCommand{}, errors.New("/plan requires a prompt")
		}
		return AgentCommand{Kind: AgentCommandRun, AgentName: AgentPlan, Prompt: prompt}, nil
	}

	if strings.HasPrefix(text, "/agent ") {
		return parseRunAgentCommand(text)
	}

	if strings.HasPrefix(text, "/parallel ") {
		return parseParallelCommand(text)
	}

	if strings.HasPrefix(text, "/") {
		return AgentCommand{}, fmt.Errorf("unknown command %q", firstCommandToken(text))
	}

	return AgentCommand{Kind: AgentCommandChat, Prompt: text}, nil
}

func parseRunAgentCommand(text string) (AgentCommand, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(text, "/agent "))
	agentName, prompt, ok := strings.Cut(rest, " ")
	if !ok {
		return AgentCommand{}, errors.New("/agent requires an agent name and a prompt")
	}
	agentName = strings.TrimSpace(agentName)
	prompt = strings.TrimSpace(prompt)
	if agentName == "" || prompt == "" {
		return AgentCommand{}, errors.New("/agent requires an agent name and a prompt")
	}
	return AgentCommand{Kind: AgentCommandRun, AgentName: agentName, Prompt: prompt}, nil
}

func parseParallelCommand(text string) (AgentCommand, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(text, "/parallel "))
	agentName, rawTasks, ok := strings.Cut(rest, " ")
	if !ok {
		return AgentCommand{}, errors.New("/parallel requires an agent name and at least one task")
	}
	agentName = strings.TrimSpace(agentName)
	if agentName == "" {
		return AgentCommand{}, errors.New("/parallel requires an agent name")
	}

	taskParts := strings.Split(rawTasks, "||")
	tasks := make([]string, 0, len(taskParts))
	for _, task := range taskParts {
		task = strings.TrimSpace(task)
		if task == "" {
			continue
		}
		tasks = append(tasks, task)
	}
	if len(tasks) == 0 {
		return AgentCommand{}, errors.New("/parallel requires at least one non-empty task")
	}
	return AgentCommand{Kind: AgentCommandParallel, AgentName: agentName, Tasks: tasks}, nil
}

func firstCommandToken(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return text
	}
	return fields[0]
}
