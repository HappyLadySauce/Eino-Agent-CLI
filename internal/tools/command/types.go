package command

// RunInput runs a local command.
// RunInput 运行本地命令。
type RunInput struct {
	Command        string `json:"command" jsonschema:"required"`
	CWD            string `json:"cwd,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	DryRun         bool   `json:"dry_run,omitempty"`
}

// RunOutput contains command execution results.
// RunOutput 包含命令执行结果。
type RunOutput struct {
	Command   string `json:"command"`
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Truncated bool   `json:"truncated"`
	Risk      string `json:"risk"`
	DryRun    bool   `json:"dry_run"`
}
