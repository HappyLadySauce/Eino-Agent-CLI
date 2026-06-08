package options

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/pflag"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
)

// SecurityOptions stores sandbox and approval configuration.
// SecurityOptions 存储沙箱和审批配置。
type SecurityOptions struct {
	SandboxMode  string `mapstructure:"sandbox-mode"`
	ApprovalMode string `mapstructure:"approval-mode"`
	DataDir      string `mapstructure:"data-dir"`
}

// NewSecurityOptions creates default security options.
// NewSecurityOptions 创建默认安全选项。
func NewSecurityOptions() *SecurityOptions {
	return &SecurityOptions{
		SandboxMode:  string(security.SandboxModeWorkspaceWrite),
		ApprovalMode: string(security.ApprovalModeInteractive),
		DataDir:      defaultEinoDataDir(),
	}
}

// AddFlags registers security flags.
// AddFlags 注册安全相关命令行标志。
func (o *SecurityOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.SandboxMode, "sandbox-mode", o.SandboxMode, "Security sandbox mode: read-only, workspace-write, or danger-full-access.")
	fs.StringVar(&o.ApprovalMode, "approval-mode", o.ApprovalMode, "Approval mode: interactive, auto, or never.")
	fs.StringVar(&o.DataDir, "data-dir", o.DataDir, "Eino data directory for sessions, audit logs, memory, and rules.")
}

// Validate validates security options.
// Validate 校验安全选项。
func (o *SecurityOptions) Validate() error {
	if o == nil {
		return errors.New("security options are required")
	}
	if !security.IsKnownSandboxMode(security.SandboxMode(o.SandboxMode)) {
		return fmt.Errorf("unsupported sandbox mode %q", o.SandboxMode)
	}
	if !security.IsKnownApprovalMode(security.ApprovalMode(o.ApprovalMode)) {
		return fmt.Errorf("unsupported approval mode %q", o.ApprovalMode)
	}
	if o.DataDir == "" {
		return errors.New("security data-dir is required")
	}
	return nil
}

func defaultEinoDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", ".eino")
	}
	return filepath.Join(home, "eino")
}
