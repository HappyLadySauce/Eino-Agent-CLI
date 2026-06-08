package agents

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/approval"
)

func TestInputRouterRoutesApprovalBeforeChat(t *testing.T) {
	t.Parallel()

	reader, writer := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = writer.Close()
		_ = reader.Close()
	})

	router := NewInputRouter(ctx, reader)
	prompter := approval.NewCLIPrompter(router, io.Discard)

	approvalDone := make(chan struct{})
	go func() {
		defer close(approvalDone)
		if _, err := io.WriteString(writer, "y\n"); err != nil {
			t.Errorf("write approval input: %v", err)
		}
	}()

	decision, err := prompter.Prompt(ctx, approval.Request{ToolName: "create_file", Reason: "test"})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if decision != approval.DecisionApproveOnce {
		t.Fatalf("decision = %q, want approve_once", decision)
	}
	<-approvalDone

	if _, err := io.WriteString(writer, "chat-message\n"); err != nil {
		t.Fatalf("write chat input: %v", err)
	}

	prompt, ok := receivePrompt(ctx, router.ChatPrompts())
	if !ok {
		t.Fatal("chat channel closed unexpectedly")
	}
	if prompt.err != nil {
		t.Fatalf("chat prompt error = %v", prompt.err)
	}
	if prompt.text != "chat-message" {
		t.Fatalf("chat prompt = %q, want chat-message", prompt.text)
	}
}

func TestInputRouterDrainsStaleApprovalInput(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	router := NewInputRouter(ctx, strings.NewReader("y\n\nsecond-chat\n"))
	prompter := approval.NewCLIPrompter(router, io.Discard)

	if _, err := prompter.Prompt(ctx, approval.Request{ToolName: "create_file", Reason: "first"}); err != nil {
		t.Fatalf("first Prompt() error = %v", err)
	}

	deadline := time.After(200 * time.Millisecond)
	for {
		select {
		case prompt, ok := <-router.ChatPrompts():
			if !ok {
				t.Fatal("chat channel closed unexpectedly")
			}
			if prompt.err != nil {
				t.Fatalf("chat prompt error = %v", prompt.err)
			}
			if prompt.text == "second-chat" {
				return
			}
			t.Fatalf("unexpected chat prompt = %q", prompt.text)
		case <-deadline:
			t.Fatal("timed out waiting for routed chat input")
		}
	}
}
