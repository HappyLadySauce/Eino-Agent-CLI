package security

import (
	"errors"
	"strings"
	"unicode"
)

// CommandClassification is the security view of one command string.
// CommandClassification 是单条命令字符串的安全视图。
type CommandClassification struct {
	Tokens      []string      // 令牌
	Simple      bool          // 是否简单
	Risk        OperationRisk // 风险等级
	ReadOnly    bool          // 是否只读
	Network     bool          // 是否网络访问
	WritesFiles bool          // 是否写文件
	Reasons     []string      // 原因
}

// ClassifyCommand classifies a shell command using syntax, executable, and argument risk.
// ClassifyCommand 使用语法、可执行文件和参数风险对 shell 命令分类。
func ClassifyCommand(command string) CommandClassification {
	tokens, complex, reasons, err := tokenizeCommand(command)
	if err != nil {
		return CommandClassification{
			Simple:  false,
			Risk:    OperationRiskUnknown,
			Reasons: []string{err.Error()},
		}
	}
	if len(tokens) == 0 {
		return CommandClassification{
			Simple:  false,
			Risk:    OperationRiskUnknown,
			Reasons: []string{"empty command"},
		}
	}
	classification := CommandClassification{
		Tokens:  tokens,
		Simple:  !complex,
		Risk:    OperationRiskMedium,
		Reasons: reasons,
	}
	executable := normalizeExecutable(tokens[0])
	applyExecutableRisk(&classification, executable, tokens)
	applyArgumentRisk(&classification, executable, tokens)
	if complex {
		classification.Risk = maxRisk(classification.Risk, OperationRiskHigh)
	}
	if classification.Network && containsShellExecution(tokens) {
		classification.Risk = OperationRiskHigh
		classification.Reasons = append(classification.Reasons, "network output can execute shell code")
	}
	if classification.ReadOnly && classification.Simple && len(classification.Reasons) == 0 {
		classification.Risk = OperationRiskLow
	}
	return classification
}

func tokenizeCommand(command string) ([]string, bool, []string, error) {
	var tokens []string
	var current strings.Builder
	var reasons []string
	var quote rune
	complex := false
	escaped := false
	text := strings.TrimSpace(command)
	if text == "" {
		return nil, false, nil, errors.New("empty command")
	}
	for i, r := range text {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '`' {
			if quote == 0 {
				complex = true
				reasons = append(reasons, "backtick outside quotes")
			}
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		if unicode.IsSpace(r) {
			flushToken(&tokens, &current)
			continue
		}
		if isComplexOperator(text, i, r) {
			flushToken(&tokens, &current)
			complex = true
			reasons = append(reasons, "shell control operator")
			continue
		}
		current.WriteRune(r)
	}
	if quote != 0 {
		return nil, false, nil, errors.New("unterminated quoted string")
	}
	flushToken(&tokens, &current)
	return tokens, complex, reasons, nil
}

func flushToken(tokens *[]string, current *strings.Builder) {
	if current.Len() == 0 {
		return
	}
	*tokens = append(*tokens, current.String())
	current.Reset()
}

func isComplexOperator(text string, index int, r rune) bool {
	switch r {
	case ';', '|', '>', '<', '&':
		return true
	case '$':
		return index+1 < len(text) && text[index+1] == '('
	case '@':
		return index+1 < len(text) && text[index+1] == '('
	case '{', '}':
		return true
	default:
		return false
	}
}

func normalizeExecutable(token string) string {
	executable := strings.ToLower(strings.TrimSpace(token))
	executable = strings.Trim(executable, `"'`)
	switch executable {
	case "rm", "del", "erase":
		return "remove-item"
	case "mv":
		return "move-item"
	case "cp":
		return "copy-item"
	case "cat":
		return "get-content"
	case "curl", "wget", "iwr":
		return "invoke-webrequest"
	case "iex":
		return "invoke-expression"
	default:
		return executable
	}
}

func applyExecutableRisk(classification *CommandClassification, executable string, tokens []string) {
	switch executable {
	case "git":
		classifyGit(classification, tokens)
	case "go":
		classifyGo(classification, tokens)
	case "pwd", "dir", "ls", "get-childitem", "get-location":
		classification.ReadOnly = true
	case "invoke-webrequest", "invoke-restmethod", "curl", "wget":
		classification.Network = true
		classification.Risk = maxRisk(classification.Risk, OperationRiskHigh)
		classification.Reasons = append(classification.Reasons, "network-capable command")
	case "invoke-expression", "powershell", "pwsh", "cmd", "bash", "wsl", "sh":
		classification.Risk = OperationRiskHigh
		classification.Reasons = append(classification.Reasons, "nested shell or dynamic execution")
	case "remove-item", "rmdir", "rd":
		classification.Risk = OperationRiskDestructive
		classification.WritesFiles = true
		classification.Reasons = append(classification.Reasons, "destructive command")
	case "new-item", "set-content", "add-content", "out-file", "copy-item", "move-item", "rename-item", "set-item":
		classification.Risk = OperationRiskHigh
		classification.WritesFiles = true
		classification.Reasons = append(classification.Reasons, "state-changing command")
	default:
		classification.Risk = OperationRiskUnknown
		classification.Reasons = append(classification.Reasons, "unknown command")
	}
}

func classifyGit(classification *CommandClassification, tokens []string) {
	if len(tokens) < 2 {
		classification.Risk = OperationRiskUnknown
		classification.Reasons = append(classification.Reasons, "git subcommand is missing")
		return
	}
	switch strings.ToLower(tokens[1]) {
	case "status", "diff", "log", "show", "branch":
		classification.ReadOnly = true
	case "fetch":
		classification.Network = true
		classification.Risk = OperationRiskMedium
		classification.Reasons = append(classification.Reasons, "git fetch accesses network")
	case "push", "pull", "merge", "rebase", "commit", "checkout", "switch", "reset", "clean":
		classification.WritesFiles = true
		classification.Risk = OperationRiskHigh
		classification.Reasons = append(classification.Reasons, "git command changes local or remote state")
	default:
		classification.Risk = OperationRiskUnknown
		classification.Reasons = append(classification.Reasons, "unknown git subcommand")
	}
}

func classifyGo(classification *CommandClassification, tokens []string) {
	if len(tokens) < 2 {
		classification.Risk = OperationRiskUnknown
		classification.Reasons = append(classification.Reasons, "go subcommand is missing")
		return
	}
	switch strings.ToLower(tokens[1]) {
	case "test", "version", "env", "list":
		classification.ReadOnly = true
	case "build":
		classification.WritesFiles = true
		classification.Risk = OperationRiskMedium
		classification.Reasons = append(classification.Reasons, "go build may write output files")
	case "get", "install", "mod":
		classification.Network = true
		classification.WritesFiles = true
		classification.Risk = OperationRiskHigh
		classification.Reasons = append(classification.Reasons, "go command can modify dependencies or access network")
	default:
		classification.Risk = OperationRiskUnknown
		classification.Reasons = append(classification.Reasons, "unknown go subcommand")
	}
}

func applyArgumentRisk(classification *CommandClassification, executable string, tokens []string) {
	for i, token := range tokens {
		lower := strings.ToLower(token)
		if lower == "-encodedcommand" || lower == "-enc" {
			classification.Risk = OperationRiskHigh
			classification.Reasons = append(classification.Reasons, "encoded command is not auto-approved")
		}
		if lower == "-command" && i+1 < len(tokens) {
			classification.Risk = OperationRiskHigh
			classification.Reasons = append(classification.Reasons, "inline command payload is not auto-approved")
		}
		if lower == "-o" || strings.HasPrefix(lower, "-o=") || lower == "--output" || strings.HasPrefix(lower, "--output=") {
			classification.WritesFiles = true
			classification.Risk = maxRisk(classification.Risk, OperationRiskMedium)
			classification.Reasons = append(classification.Reasons, "command declares output path")
		}
		if (executable == "python" && lower == "-c") || (executable == "node" && lower == "-e") {
			classification.Risk = OperationRiskHigh
			classification.Reasons = append(classification.Reasons, "inline interpreter payload is not auto-approved")
		}
	}
}

func containsShellExecution(tokens []string) bool {
	for _, token := range tokens {
		switch normalizeExecutable(token) {
		case "sh", "bash", "cmd", "powershell", "pwsh", "invoke-expression":
			return true
		}
	}
	return false
}

func maxRisk(a, b OperationRisk) OperationRisk {
	order := map[OperationRisk]int{
		OperationRiskLow:         1,
		OperationRiskMedium:      2,
		OperationRiskHigh:        3,
		OperationRiskDestructive: 4,
		OperationRiskUnknown:     5,
	}
	if order[b] > order[a] {
		return b
	}
	return a
}
