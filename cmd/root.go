package main

import (
	"context"
	"os"

	"k8s.io/component-base/cli"

	"github.com/HappyLadySauce/Eino-Agent-CLI/cmd/app"
)

const (
	basename = "eino"
)

func main() {
	ctx := context.TODO()
	cmd := app.NewAPICommand(ctx, basename)
	code := cli.Run(cmd)
	os.Exit(code)
}