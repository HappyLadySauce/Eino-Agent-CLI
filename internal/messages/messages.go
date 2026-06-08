package messages

import (
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
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
		if messageOutput.Role != schema.Assistant {
			continue
		}

		if messageOutput.IsStreaming && messageOutput.MessageStream != nil {
			chunkCount, err := consumeMessageStream(messageOutput.MessageStream, writer, &reply)
			result.ChunkCount += chunkCount
			if err != nil {
				return result, fmt.Errorf("assistant message stream failed: %w", err)
			}
			continue
		}

		if err := consumeMessageEvent(event, writer, &reply); err != nil {
			return result, fmt.Errorf("assistant message event failed: %w", err)
		}
	}

	result.Content = reply.String()
	return result, nil
}

// consumeMessageStream writes streaming message chunks and appends them to reply.
// consumeMessageStream 写出流式消息分片，并将内容追加到 reply。
func consumeMessageStream(stream *schema.StreamReader[*schema.Message], writer io.Writer, reply *strings.Builder) (int, error) {
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
		if chunk == nil || chunk.Content == "" {
			continue
		}

		if _, err := io.WriteString(writer, chunk.Content); err != nil {
			return chunkCount, fmt.Errorf("failed to write assistant stream chunk: %w", err)
		}
		reply.WriteString(chunk.Content)
		chunkCount++
	}
}

// consumeMessageEvent writes a non-streaming assistant message and appends it to reply.
// consumeMessageEvent 写出非流式 assistant 消息，并将内容追加到 reply。
func consumeMessageEvent(event *adk.AgentEvent, writer io.Writer, reply *strings.Builder) error {
	msg, _, err := adk.GetMessage(event)
	if err != nil {
		return fmt.Errorf("failed to extract assistant message: %w", err)
	}
	if msg == nil || msg.Content == "" {
		return nil
	}

	if _, err := io.WriteString(writer, msg.Content); err != nil {
		return fmt.Errorf("failed to write assistant message: %w", err)
	}
	reply.WriteString(msg.Content)
	return nil
}
