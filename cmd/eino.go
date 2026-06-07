package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/component-base/cli"

	"github.com/HappyLadySauce/Eino-Agent-CLI/cmd/app"
)

const (
	basename = "eino"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	cmd := app.NewAPICommand(ctx, basename)
	code := cli.Run(cmd)
	stop()
	os.Exit(code)
}
