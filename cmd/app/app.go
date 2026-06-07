package app

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/agents"

	"github.com/HappyLadySauce/Eino-Agent-CLI/cmd/app/options"
	"github.com/HappyLadySauce/Eino-Agent-CLI/pkg/config"
	pkgoptions "github.com/HappyLadySauce/Eino-Agent-CLI/pkg/options"
)

func NewAPICommand(ctx context.Context, basename string) *cobra.Command {
	opts := options.NewOptions(basename)
	cmd := &cobra.Command{
		Use:   basename,
		Short: basename + " is a web server",
		Long:  basename + " is a web server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Bind command-line flags to Viper (CLI values override the config file).
			// 将命令行标志绑定到 Viper（命令行参数覆盖配置文件）。
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}

			if err := viper.Unmarshal(opts); err != nil {
				return err
			}

			// Keep the loaded config file path for user-facing validation errors.
			// 保留已加载配置文件路径，供后续面向用户的校验报错使用。
			opts.SetConfigPath(pkgoptions.LoadedConfigPath())

			// Initialize logging after flags are parsed and configuration is loaded.
			// 在解析完标志并加载配置后初始化日志。
			logs.InitLogs()
			defer logs.FlushLogs()

			// Validate options after flags and configuration are fully populated.
			// 在标志与配置全部就绪后校验选项。
			if err := opts.Validate(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Configuration file: %s\n", opts.ConfigPath())
				_, _ = fmt.Fprintf(os.Stderr, "%s\n", err)
				return fmt.Errorf("configuration incomplete")
			}
			return run(ctx, opts)
		},
	}

	nfs := opts.AddFlags(cmd.Flags())
	flag.SetUsageAndHelpFunc(cmd, *nfs, 80)

	return cmd
}

func run(ctx context.Context, opts *options.Options) error {
	cfg := &config.Config{
		Model: opts.Model,
	}
	config.Init(cfg)

	err := agents.RunAgentLoop(ctx, cfg)
	if err != nil {
		return err
	}

	return nil
}
