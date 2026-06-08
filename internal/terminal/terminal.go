package terminal

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	ansiReset   = "\x1b[0m"
	ansiBold    = "\x1b[1m"
	ansiDim     = "\x1b[2m"
	ansiBlue    = "\x1b[34m"
	ansiCyan    = "\x1b[36m"
	ansiGreen   = "\x1b[32m"
	ansiMagenta = "\x1b[35m"
	ansiYellow  = "\x1b[33m"
	clearLine   = "\r\x1b[2K"
)

type colorAware interface {
	ColorEnabled() bool
}

// Style applies ANSI colors only when the destination supports interactive output.
// Style 仅在目标支持交互式输出时应用 ANSI 颜色。
type Style struct {
	enabled bool
}

// StyleForWriter returns a terminal style policy for writer.
// Parameters:
//   - writer: destination writer used by the CLI.
//
// Returns:
//   - Style: color policy that is disabled for non-TTY writers.
//
// Example:
//
//	style := terminal.StyleForWriter(os.Stdout)
//	fmt.Fprint(os.Stdout, style.UserPrompt("User[agent]> "))
//
// StyleForWriter 返回 writer 对应的终端样式策略。
// 参数：
//   - writer：CLI 输出目标。
//
// 返回值：
//   - Style：非 TTY writer 会自动禁用颜色。
//
// 使用示例：
//
//	style := terminal.StyleForWriter(os.Stdout)
//	fmt.Fprint(os.Stdout, style.UserPrompt("User[agent]> "))
func StyleForWriter(writer io.Writer) Style {
	return Style{enabled: SupportsColor(writer)}
}

// SupportsColor reports whether ANSI colors should be emitted for writer.
// SupportsColor 判断是否应该为 writer 输出 ANSI 颜色。
func SupportsColor(writer io.Writer) bool {
	if aware, ok := writer.(colorAware); ok {
		return aware.ColorEnabled()
	}
	if os.Getenv("NO_COLOR") != "" || strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	file, ok := writer.(*os.File)
	if !ok || file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// Enabled returns true when ANSI styling is active.
// Enabled 返回 ANSI 样式是否启用。
func (s Style) Enabled() bool {
	return s.enabled
}

// UserPrompt styles the user prompt.
// UserPrompt 设置用户提示符样式。
func (s Style) UserPrompt(text string) string {
	return s.wrap(ansiBold+ansiBlue, text)
}

// Assistant styles the final assistant prefix.
// Assistant 设置 assistant 最终输出前缀样式。
func (s Style) Assistant(text string) string {
	return s.wrap(ansiBold+ansiGreen, text)
}

// Thinking styles reasoning output.
// Thinking 设置 reasoning 输出样式。
func (s Style) Thinking(text string) string {
	return s.wrap(ansiDim+ansiMagenta, text)
}

// Tools styles tool-call output.
// Tools 设置工具调用输出样式。
func (s Style) Tools(text string) string {
	return s.wrap(ansiCyan, text)
}

// Stats styles token statistics output.
// Stats 设置 token 统计输出样式。
func (s Style) Stats(text string) string {
	return s.wrap(ansiDim+ansiYellow, text)
}

func (s Style) wrap(prefix, text string) string {
	if !s.enabled || text == "" {
		return text
	}
	return prefix + text + ansiReset
}

// AnimatedWriter displays a lightweight spinner until the first real write.
// AnimatedWriter 在首次真实输出前显示轻量 spinner。
type AnimatedWriter struct {
	writer io.Writer
	label  string
	style  Style

	writeMu sync.Mutex
	stateMu sync.Mutex
	stop    chan struct{}
	done    chan struct{}
	stopped bool
}

// NewAnimatedWriter creates and starts an animated writer when writer is a TTY.
// Parameters:
//   - writer: destination writer.
//   - label: short status label displayed beside the spinner.
//
// Returns:
//   - *AnimatedWriter: writer wrapper. Call Close after the operation finishes.
//
// Example:
//
//	out := terminal.NewAnimatedWriter(os.Stdout, "Thinking")
//	defer out.Close()
//
// NewAnimatedWriter 在 writer 是 TTY 时创建并启动动画 writer。
// 参数：
//   - writer：输出目标。
//   - label：spinner 旁边显示的短状态文本。
//
// 返回值：
//   - *AnimatedWriter：writer 包装器。操作结束后调用 Close。
//
// 使用示例：
//
//	out := terminal.NewAnimatedWriter(os.Stdout, "Thinking")
//	defer out.Close()
func NewAnimatedWriter(writer io.Writer, label string) *AnimatedWriter {
	if writer == nil {
		writer = io.Discard
	}
	out := &AnimatedWriter{
		writer: writer,
		label:  strings.TrimSpace(label),
		style:  StyleForWriter(writer),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	if !out.style.Enabled() {
		out.stopped = true
		close(out.done)
		return out
	}
	go out.spin()
	return out
}

// ColorEnabled returns the wrapped writer color policy.
// ColorEnabled 返回被包装 writer 的颜色策略。
func (w *AnimatedWriter) ColorEnabled() bool {
	return w != nil && w.style.Enabled()
}

// Write stops the spinner before writing real output.
// Write 在写入真实输出前停止 spinner。
func (w *AnimatedWriter) Write(p []byte) (int, error) {
	if w == nil {
		return 0, io.ErrClosedPipe
	}
	if err := w.Close(); err != nil {
		return 0, err
	}
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	return w.writer.Write(p)
}

// Close stops and clears the spinner.
// Close 停止并清理 spinner。
func (w *AnimatedWriter) Close() error {
	if w == nil {
		return nil
	}
	w.stateMu.Lock()
	if w.stopped {
		w.stateMu.Unlock()
		return nil
	}
	w.stopped = true
	close(w.stop)
	w.stateMu.Unlock()

	<-w.done
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	_, err := io.WriteString(w.writer, clearLine)
	return err
}

func (w *AnimatedWriter) spin() {
	defer close(w.done)
	frames := []byte{'|', '/', '-', '\\'}
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	index := 0
	for {
		w.writeMu.Lock()
		_, _ = fmt.Fprintf(w.writer, "%s%s", clearLine, w.style.Thinking(fmt.Sprintf("%s %c", w.label, frames[index%len(frames)])))
		w.writeMu.Unlock()
		index++

		select {
		case <-w.stop:
			return
		case <-ticker.C:
		}
	}
}
