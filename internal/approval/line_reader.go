package approval

import (
	"bufio"
	"context"
	"io"
	"strings"
)

// LineReader supplies one stdin line for interactive approval prompts.
// LineReader 为交互式审批提示提供一行 stdin 输入。
type LineReader interface {
	BeginApproval()
	ReadApprovalLine(ctx context.Context) (string, error)
	EndApproval()
}

// streamLineReader reads directly from an io.Reader for tests and non-multiplexed use.
// streamLineReader 直接从 io.Reader 读取，用于测试与无需复用的场景。
type streamLineReader struct {
	reader *bufio.Reader
}

// NewStreamLineReader wraps a reader for standalone CLI approval prompts.
// NewStreamLineReader 包装 reader，用于独立 CLI 审批提示。
func NewStreamLineReader(reader io.Reader) LineReader {
	return &streamLineReader{reader: bufio.NewReader(reader)}
}

func (s *streamLineReader) BeginApproval() {}

func (s *streamLineReader) EndApproval() {}

func (s *streamLineReader) ReadApprovalLine(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	line, err := s.reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSuffix(line, "\n"), nil
}
