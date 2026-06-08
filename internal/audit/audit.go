package audit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
)

var secretPattern = regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password|authorization|bearer|credential)=?[^,\s"']+`)

// Record is one append-only audit entry.
// Record 是一条追加式审计记录。
type Record struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	SessionID    string                 `json:"session_id"`
	SessionMode  security.SessionMode   `json:"session_mode"`
	SandboxMode  security.SandboxMode   `json:"sandbox_mode"`
	ApprovalMode security.ApprovalMode  `json:"approval_mode"`
	ToolName     string                 `json:"tool_name"`
	Provider     security.ToolProvider  `json:"provider"`
	Operation    security.OperationKind `json:"operation"`
	CWD          string                 `json:"cwd,omitempty"`
	TargetPath   string                 `json:"target_path,omitempty"`
	Arguments    string                 `json:"arguments_summary,omitempty"`
	Risk         security.OperationRisk `json:"risk"`
	Decision     security.Decision      `json:"decision"`
	Reason       string                 `json:"reason,omitempty"`
	UserDecision string                 `json:"user_decision,omitempty"`
	Duration     time.Duration          `json:"duration"`
	ResultStatus security.ResultStatus  `json:"result_status"`
}

// Sink appends audit records.
// Sink 追加审计记录。
type Sink interface {
	Append(ctx context.Context, record Record) error
}

// MemorySink keeps audit records in memory.
// MemorySink 将审计记录保存在内存中。
type MemorySink struct {
	mu      sync.Mutex
	records []Record
}

// NewMemorySink creates an in-memory audit sink.
// NewMemorySink 创建内存审计 sink。
func NewMemorySink() *MemorySink {
	return &MemorySink{}
}

// Append stores one record.
// Append 存储一条记录。
func (s *MemorySink) Append(ctx context.Context, record Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
	return nil
}

// Records returns a copy of stored audit records.
// Records 返回审计记录副本。
func (s *MemorySink) Records() []Record {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Record, len(s.records))
	copy(out, s.records)
	return out
}

// FileSink appends audit records as JSON lines.
// FileSink 将审计记录以 JSON Lines 追加到文件。
type FileSink struct {
	path string
	mu   sync.Mutex
}

// NewFileSink creates a file-backed audit sink.
// NewFileSink 创建文件审计 sink。
func NewFileSink(path string) *FileSink {
	return &FileSink{path: path}
}

// Append writes one JSON line.
// Append 写入一行 JSON。
func (s *FileSink) Append(ctx context.Context, record Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || strings.TrimSpace(s.path) == "" {
		return fmt.Errorf("audit file path is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create audit directory: %w", err)
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open audit file: %w", err)
	}
	defer file.Close()
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal audit record: %w", err)
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("write audit record: %w", err)
	}
	return nil
}

// MultiSink appends records to multiple sinks.
// MultiSink 将记录追加到多个 sink。
type MultiSink []Sink

// Append writes to all sinks.
// Append 写入所有 sink。
func (s MultiSink) Append(ctx context.Context, record Record) error {
	for _, sink := range s {
		if sink == nil {
			continue
		}
		if err := sink.Append(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

// NewID creates a stable audit id.
// NewID 创建稳定审计 ID。
func NewID() string {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("audit_%d", time.Now().UnixNano())
	}
	return "audit_" + hex.EncodeToString(bytes[:])
}

// Redact removes secret-looking values.
// Redact 移除疑似敏感值。
func Redact(text string) string {
	if text == "" {
		return ""
	}
	return secretPattern.ReplaceAllString(text, "$1=<redacted>")
}

// Summarize limits and redacts arguments.
// Summarize 限制并脱敏参数摘要。
func Summarize(text string, max int) string {
	text = Redact(strings.TrimSpace(text))
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max] + "...<truncated>"
}

// WriteJSON writes a redacted compact JSON summary.
// WriteJSON 写入脱敏的紧凑 JSON 摘要。
func WriteJSON(writer io.Writer, value any) {
	encoded, err := json.Marshal(value)
	if err != nil {
		fmt.Fprint(writer, "<unserializable>")
		return
	}
	fmt.Fprint(writer, Summarize(string(encoded), 512))
}
