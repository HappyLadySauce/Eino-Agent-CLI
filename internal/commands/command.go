package commands

import (
	"errors"
	"fmt"
	"strings"
)

// SessionMode controls how the main agent handles subsequent user input.
// SessionMode 控制主 Agent 如何处理后续用户输入。
type SessionMode string

const (
	SessionModeAgent SessionMode = "agent"
	SessionModePlan   SessionMode = "plan"
	SessionModeAsk    SessionMode = "ask"
)

// AgentCommandKind identifies one CLI agent command.
// AgentCommandKind 标识一个 CLI Agent 命令。
type AgentCommandKind int

const (
	AgentCommandChat AgentCommandKind = iota
	AgentCommandExit
	AgentCommandModeSwitch
	AgentCommandSubAgent
)

// AgentCommand is the parsed representation of one user input line.
// AgentCommand 是用户输入行的解析结果。
type AgentCommand struct {
	Kind   AgentCommandKind
	Mode   SessionMode
	Prompt string
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
	case "/agent":
		return AgentCommand{Kind: AgentCommandModeSwitch, Mode: SessionModeAgent}, nil
	case "/plan":
		return AgentCommand{Kind: AgentCommandModeSwitch, Mode: SessionModePlan}, nil
	case "/ask":
		return AgentCommand{Kind: AgentCommandModeSwitch, Mode: SessionModeAsk}, nil
	}

	if strings.HasPrefix(text, "/subagent ") {
		task := strings.TrimSpace(strings.TrimPrefix(text, "/subagent "))
		if task == "" {
			return AgentCommand{}, errors.New("/subagent requires a task")
		}
		return AgentCommand{Kind: AgentCommandSubAgent, Prompt: task}, nil
	}

	if strings.HasPrefix(text, "/") {
		return AgentCommand{}, fmt.Errorf("unknown command %q", firstCommandToken(text))
	}

	return AgentCommand{Kind: AgentCommandChat, Prompt: text}, nil
}

func firstCommandToken(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return text
	}
	return fields[0]
}
