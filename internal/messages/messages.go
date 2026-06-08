package messages

import (
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/terminal"
)

// Message Manager, used to manage conversation history.
// 消息管理器，用于管理会话历史。
type Messages struct {
	messages []*schema.Message
}

// AssistantStreamResult contains the materialized assistant response after streaming.
// AssistantStreamResult 包含流式处理后聚合得到的 assistant 回复。
type AssistantStreamResult struct {
	Content    string
	EventCount int
	ChunkCount int
}

type outputChannel int

const (
	outputChannelNone outputChannel = iota
	outputChannelFinal
	outputChannelThinking
	outputChannelTools
)

type channelWriter struct {
	writer  io.Writer
	style   terminal.Style
	current outputChannel
	wrote   bool
}

// NewMessages creates a new message manager.
// NewMessages 创建新的消息管理器。
func NewMessages() *Messages {
	return &Messages{
		messages: []*schema.Message{},
	}
}

// Add stores non-nil messages.
// Parameters:
//   - message: one or more schema messages to append.
//
// Returns:
//   - error: reserved for future validation failures. The current implementation always returns nil.
//
// Example:
//
//	msgs.Add(schema.UserMessage("hello"))
//
// Add 存储非空消息。
// 参数：
//   - message：一个或多个待追加的 schema 消息。
//
// 返回值：
//   - error：为未来校验失败保留；当前实现始终返回 nil。
//
// 使用示例：
//
//	msgs.Add(schema.UserMessage("hello"))
func (m *Messages) Add(message ...*schema.Message) error {
	if m == nil {
		return nil
	}

	for _, msg := range message {
		if msg == nil {
			continue
		}
		m.messages = append(m.messages, msg)
	}

	return nil
}

// Get returns a defensive copy of the managed messages.
// Get 返回消息列表的防御性副本。
func (m *Messages) Get() []*schema.Message {
	if m == nil || len(m.messages) == 0 {
		return nil
	}

	copied := make([]*schema.Message, len(m.messages))
	copy(copied, m.messages)
	return copied
}

// ConsumeAssistantStream consumes ADK events, writes assistant tokens, and returns the full reply.
// Parameters:
//   - iter: ADK async iterator returned by runner.Run.
//   - writer: destination for incremental assistant output. nil means io.Discard.
//
// Returns:
//   - *AssistantStreamResult: aggregated assistant content and basic processing counters.
//   - error: event error, stream read error, message extraction error, or writer error.
//
// Example:
//
//	result, err := messages.ConsumeAssistantStream(runner.Run(ctx, history), os.Stdout)
//	if err == nil && result.Content != "" {
//	    history.Add(schema.AssistantMessage(result.Content, nil))
//	}
//
// ConsumeAssistantStream 消费 ADK 事件、写出 assistant token，并返回完整回复。
// 参数：
//   - iter：runner.Run 返回的 ADK 异步迭代器。
//   - writer：增量 assistant 输出目标；nil 表示丢弃输出。
//
// 返回值：
//   - *AssistantStreamResult：聚合后的 assistant 内容和基础处理计数。
//   - error：事件错误、流读取错误、消息提取错误或写入错误。
//
// 使用示例：
//
//	result, err := messages.ConsumeAssistantStream(runner.Run(ctx, history), os.Stdout)
//	if err == nil && result.Content != "" {
//	    history.Add(schema.AssistantMessage(result.Content, nil))
//	}
func ConsumeAssistantStream(iter *adk.AsyncIterator[*adk.AgentEvent], writer io.Writer) (*AssistantStreamResult, error) {
	if writer == nil {
		writer = io.Discard
	}

	result := &AssistantStreamResult{}
	out := &channelWriter{writer: writer, style: terminal.StyleForWriter(writer)}
	var reply strings.Builder

	if iter == nil {
		return result, nil
	}

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		result.EventCount++

		if event == nil {
			continue
		}
		if event.Err != nil {
			return result, fmt.Errorf("assistant stream event failed: %w", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		messageOutput := event.Output.MessageOutput
		if messageOutput.Role != schema.Assistant && messageOutput.Role != schema.Tool {
			continue
		}

		if messageOutput.IsStreaming && messageOutput.MessageStream != nil {
			chunkCount, err := consumeMessageStream(messageOutput.MessageStream, out, &reply)
			result.ChunkCount += chunkCount
			if err != nil {
				return result, fmt.Errorf("assistant message stream failed: %w", err)
			}
			continue
		}

		if err := consumeMessageEvent(event, out, &reply); err != nil {
			return result, fmt.Errorf("assistant message event failed: %w", err)
		}
	}

	if err := out.finish(); err != nil {
		return result, fmt.Errorf("failed to finish assistant output: %w", err)
	}

	result.Content = reply.String()
	return result, nil
}

// consumeMessageStream writes streaming message chunks and appends them to reply.
// consumeMessageStream 写出流式消息分片，并将内容追加到 reply。
func consumeMessageStream(stream *schema.StreamReader[*schema.Message], writer *channelWriter, reply *strings.Builder) (int, error) {
	defer stream.Close()

	chunkCount := 0
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return chunkCount, nil
			}
			return chunkCount, fmt.Errorf("failed to receive assistant stream chunk: %w", err)
		}
		if chunk == nil {
			continue
		}

		wrote, err := consumeMessage(chunk, writer, reply)
		if err != nil {
			return chunkCount, fmt.Errorf("failed to write assistant stream chunk: %w", err)
		}
		if wrote {
			chunkCount++
		}
	}
}

// consumeMessageEvent writes a non-streaming assistant message and appends it to reply.
// consumeMessageEvent 写出非流式 assistant 消息，并将内容追加到 reply。
func consumeMessageEvent(event *adk.AgentEvent, writer *channelWriter, reply *strings.Builder) error {
	msg, _, err := adk.GetMessage(event)
	if err != nil {
		return fmt.Errorf("failed to extract assistant message: %w", err)
	}
	if msg == nil {
		return nil
	}

	_, err = consumeMessage(msg, writer, reply)
	return err
}

func consumeMessage(msg *schema.Message, writer *channelWriter, reply *strings.Builder) (bool, error) {
	wrote := false

	if msg.ReasoningContent != "" {
		if err := writer.write(outputChannelThinking, msg.ReasoningContent); err != nil {
			return wrote, err
		}
		wrote = true
	}

	for _, part := range msg.AssistantGenMultiContent {
		switch part.Type {
		case schema.ChatMessagePartTypeReasoning:
			if part.Reasoning == nil || part.Reasoning.Text == "" {
				continue
			}
			if err := writer.write(outputChannelThinking, part.Reasoning.Text); err != nil {
				return wrote, err
			}
			wrote = true
		case schema.ChatMessagePartTypeText:
			partWrote, err := writeAssistantContent(writer, reply, part.Text)
			if err != nil {
				return wrote, err
			}
			wrote = wrote || partWrote
		}
	}

	if len(msg.ToolCalls) > 0 {
		for _, toolCall := range msg.ToolCalls {
			if err := writer.writeLine(outputChannelTools, formatToolCall(toolCall)); err != nil {
				return wrote, err
			}
			wrote = true
		}
	}

	if msg.Role == schema.Tool && msg.Content != "" {
		if err := writer.writeLine(outputChannelTools, formatToolResult(msg)); err != nil {
			return wrote, err
		}
		wrote = true
	}

	if msg.Role == schema.Assistant {
		contentWrote, err := writeAssistantContent(writer, reply, msg.Content)
		if err != nil {
			return wrote, err
		}
		wrote = wrote || contentWrote
	}
	return wrote, nil
}

func writeAssistantContent(writer *channelWriter, reply *strings.Builder, content string) (bool, error) {
	if strings.TrimSpace(content) == "" {
		return false, nil
	}

	wrote := false
	for _, segment := range splitAssistantContent(content) {
		text := segment.text
		if text == "" {
			continue
		}
		channel := outputChannelFinal
		if segment.thinking {
			channel = outputChannelThinking
		} else {
			reply.WriteString(text)
		}
		if err := writer.write(channel, text); err != nil {
			return wrote, err
		}
		wrote = true
	}
	return wrote, nil
}

type assistantContentSegment struct {
	text     string
	thinking bool
}

func splitAssistantContent(content string) []assistantContentSegment {
	const startMarker = "<|channel>thought"
	const endMarker = "<channel|>"

	var segments []assistantContentSegment
	remaining := content
	for {
		start := strings.Index(remaining, startMarker)
		if start < 0 {
			if remaining != "" {
				segments = append(segments, assistantContentSegment{text: remaining})
			}
			return segments
		}
		if start > 0 {
			segments = append(segments, assistantContentSegment{text: remaining[:start]})
		}

		thinkingStart := start + len(startMarker)
		if strings.HasPrefix(remaining[thinkingStart:], "\r\n") {
			thinkingStart += 2
		} else if strings.HasPrefix(remaining[thinkingStart:], "\n") {
			thinkingStart++
		}
		afterStart := remaining[thinkingStart:]
		end := strings.Index(afterStart, endMarker)
		if end < 0 {
			segments = append(segments, assistantContentSegment{text: afterStart, thinking: true})
			return segments
		}

		segments = append(segments, assistantContentSegment{text: afterStart[:end], thinking: true})
		remaining = afterStart[end+len(endMarker):]
	}
}

func formatToolCall(toolCall schema.ToolCall) string {
	name := toolCall.Function.Name
	if name == "" {
		name = toolCall.ID
	}
	args := strings.TrimSpace(toolCall.Function.Arguments)
	if args == "" || args == "{}" {
		return name
	}
	return fmt.Sprintf("%s %s", name, args)
}

func formatToolResult(msg *schema.Message) string {
	name := msg.ToolName
	if name == "" {
		name = msg.ToolCallID
	}
	if name == "" {
		return msg.Content
	}
	return fmt.Sprintf("%s: %s", name, msg.Content)
}

func (w *channelWriter) write(channel outputChannel, text string) error {
	if text == "" {
		return nil
	}
	if err := w.ensureChannel(channel); err != nil {
		return err
	}
	_, err := io.WriteString(w.writer, text)
	return err
}

func (w *channelWriter) writeLine(channel outputChannel, text string) error {
	if text == "" {
		return nil
	}
	if err := w.write(channel, text); err != nil {
		return err
	}
	if _, err := io.WriteString(w.writer, "\n"); err != nil {
		return err
	}
	w.current = outputChannelNone
	return nil
}

func (w *channelWriter) ensureChannel(channel outputChannel) error {
	if w.current == channel {
		return nil
	}
	if w.current != outputChannelNone {
		if _, err := io.WriteString(w.writer, "\n"); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w.writer, w.channelPrefix(channel)); err != nil {
		return err
	}
	w.current = channel
	w.wrote = true
	return nil
}

func (w *channelWriter) finish() error {
	if !w.wrote || w.current == outputChannelNone {
		return nil
	}
	_, err := io.WriteString(w.writer, "\n")
	w.current = outputChannelNone
	return err
}

func (w *channelWriter) channelPrefix(channel outputChannel) string {
	switch channel {
	case outputChannelThinking:
		return w.style.Thinking("Assistant[thinking]> ")
	case outputChannelTools:
		return w.style.Tools("Assistant[tools]> ")
	default:
		return w.style.Assistant("Assistant> ")
	}
}
