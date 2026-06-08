package messages

import (
	"bytes"
	"errors"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestMessagesAddSkipsNilMessagesAndReturnsCopy(t *testing.T) {
	msgs := NewMessages()

	if err := msgs.Add(nil, schema.UserMessage("first"), schema.UserMessage("second"), schema.UserMessage("third")); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	got := msgs.Get()
	if len(got) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(got))
	}
	if got[0].Content != "first" || got[1].Content != "second" || got[2].Content != "third" {
		t.Fatalf("unexpected message content: got %q, %q, %q", got[0].Content, got[1].Content, got[2].Content)
	}

	got[0] = schema.UserMessage("mutated")
	again := msgs.Get()
	if again[0].Content != "first" {
		t.Fatalf("Get should return a defensive copy, got %q", again[0].Content)
	}
}

func TestMessagesAddAllowsMessagesWithoutUsage(t *testing.T) {
	msgs := NewMessages()

	if err := msgs.Add(schema.UserMessage("hello"), schema.AssistantMessage("world", nil)); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	got := msgs.Get()
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	if got[0].Content != "hello" || got[1].Content != "world" {
		t.Fatalf("unexpected messages: %#v", got)
	}
}

func TestConsumeAssistantStreamHandlesStreamingAndNonStreamingMessages(t *testing.T) {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	stream := schema.StreamReaderFromArray([]*schema.Message{
		schema.AssistantMessage("hel", nil),
		schema.AssistantMessage("lo", nil),
	})

	gen.Send(adk.EventFromMessage(nil, stream, schema.Assistant, ""))
	gen.Send(adk.EventFromMessage(schema.AssistantMessage("!", nil), nil, schema.Assistant, ""))
	gen.Send(adk.EventFromMessage(schema.ToolMessage("ignored", "tool-call-id"), nil, schema.Tool, "tool"))
	gen.Close()

	var out bytes.Buffer
	result, err := ConsumeAssistantStream(iter, &out)
	if err != nil {
		t.Fatalf("ConsumeAssistantStream returned error: %v", err)
	}

	if result.Content != "hello!" {
		t.Fatalf("expected content %q, got %q", "hello!", result.Content)
	}
	wantOutput := "Assistant> hello!\nAssistant[tools]> tool-call-id: ignored\n"
	if out.String() != wantOutput {
		t.Fatalf("expected output %q, got %q", wantOutput, out.String())
	}
	if result.EventCount != 3 {
		t.Fatalf("expected 3 events, got %d", result.EventCount)
	}
	if result.ChunkCount != 2 {
		t.Fatalf("expected 2 chunks, got %d", result.ChunkCount)
	}
}

func TestConsumeAssistantStreamSeparatesThinkingToolsAndFinalContent(t *testing.T) {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	stream := schema.StreamReaderFromArray([]*schema.Message{
		{Role: schema.Assistant, ReasoningContent: "checking tools"},
		schema.AssistantMessage("final answer", []schema.ToolCall{
			{
				ID:   "call-1",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "list_agents",
					Arguments: `{"type":"all"}`,
				},
			},
		}),
	})

	gen.Send(adk.EventFromMessage(nil, stream, schema.Assistant, ""))
	gen.Send(adk.EventFromMessage(schema.ToolMessage("tool output", "call-1", schema.WithToolName("list_agents")), nil, schema.Tool, "list_agents"))
	gen.Close()

	var out bytes.Buffer
	result, err := ConsumeAssistantStream(iter, &out)
	if err != nil {
		t.Fatalf("ConsumeAssistantStream returned error: %v", err)
	}

	if result.Content != "final answer" {
		t.Fatalf("expected final content %q, got %q", "final answer", result.Content)
	}
	wantOutput := "Assistant[thinking]> checking tools\nAssistant[tools]> list_agents {\"type\":\"all\"}\nAssistant> final answer\nAssistant[tools]> list_agents: tool output\n"
	if out.String() != wantOutput {
		t.Fatalf("unexpected output:\ngot:  %q\nwant: %q", out.String(), wantOutput)
	}
}

func TestConsumeAssistantStreamRoutesLeakedThoughtChannelToThinking(t *testing.T) {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	stream := schema.StreamReaderFromArray([]*schema.Message{
		schema.AssistantMessage("<|channel>thought\nhidden<channel|>visible", nil),
	})

	gen.Send(adk.EventFromMessage(nil, stream, schema.Assistant, ""))
	gen.Close()

	var out bytes.Buffer
	result, err := ConsumeAssistantStream(iter, &out)
	if err != nil {
		t.Fatalf("ConsumeAssistantStream returned error: %v", err)
	}

	if result.Content != "visible" {
		t.Fatalf("expected final content %q, got %q", "visible", result.Content)
	}
	wantOutput := "Assistant[thinking]> hidden\nAssistant> visible\n"
	if out.String() != wantOutput {
		t.Fatalf("unexpected output:\ngot:  %q\nwant: %q", out.String(), wantOutput)
	}
}

func TestConsumeAssistantStreamReturnsEventError(t *testing.T) {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	expectedErr := errors.New("runner failed")

	gen.Send(&adk.AgentEvent{Err: expectedErr})
	gen.Close()

	result, err := ConsumeAssistantStream(iter, nil)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected event error %v, got %v", expectedErr, err)
	}
	if result == nil || result.EventCount != 1 {
		t.Fatalf("expected partial result with one event, got %#v", result)
	}
}
