package options

import (
	"errors"
	"fmt"

	"github.com/spf13/pflag"
)

const (
	DefaultMaxParallelWorkers = 10
	MaxParallelWorkersLimit   = 16
)

// AgentOptions contains runtime options for agent orchestration.
// AgentOptions 包含 Agent 编排运行时选项。
type AgentOptions struct {
	MaxParallelWorkers int `mapstructure:"EINO_MAX_PARALLEL_WORKERS"`
}

// NewAgentOptions creates default agent orchestration options.
// NewAgentOptions 创建默认 Agent 编排选项。
func NewAgentOptions() *AgentOptions {
	return &AgentOptions{
		MaxParallelWorkers: DefaultMaxParallelWorkers,
	}
}

// Validate validates agent orchestration options.
// Validate 校验 Agent 编排选项。
func (o *AgentOptions) Validate() error {
	if o == nil {
		return errors.New("agent options are required")
	}
	if o.MaxParallelWorkers < 1 {
		return fmt.Errorf("max_parallel_workers must be greater than or equal to 1, got %d", o.MaxParallelWorkers)
	}
	if o.MaxParallelWorkers > MaxParallelWorkersLimit {
		return fmt.Errorf("max_parallel_workers must be less than or equal to %d, got %d", MaxParallelWorkersLimit, o.MaxParallelWorkers)
	}
	return nil
}

// AddFlags registers agent orchestration flags.
// AddFlags 注册 Agent 编排标志。
func (o *AgentOptions) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&o.MaxParallelWorkers, "max-parallel-workers", DefaultMaxParallelWorkers, "The maximum number of parallel agent workers")
}
