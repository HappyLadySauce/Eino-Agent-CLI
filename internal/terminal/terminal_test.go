package terminal

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestStyleForWriterDisablesColorForBuffer(t *testing.T) {
	style := StyleForWriter(&bytes.Buffer{})

	if style.Enabled() {
		t.Fatal("StyleForWriter(buffer).Enabled() = true, want false")
	}
	if got, want := style.UserPrompt("User[agent]> "), "User[agent]> "; got != want {
		t.Fatalf("UserPrompt() = %q, want %q", got, want)
	}
}

func TestSupportsColorHonorsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	if SupportsColor(os.Stdout) {
		t.Fatal("SupportsColor() = true with NO_COLOR set, want false")
	}
}

func TestAnimatedWriterWritesPlainTextForNonTTY(t *testing.T) {
	var out bytes.Buffer
	writer := NewAnimatedWriter(&out, "Thinking")

	if _, err := writer.Write([]byte("hello")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if got, want := out.String(), "hello"; got != want {
		t.Fatalf("AnimatedWriter output = %q, want %q", got, want)
	}
}

func TestStyleWrapsWhenColorAwareWriterIsEnabled(t *testing.T) {
	writer := fakeColorWriter{enabled: true}
	style := StyleForWriter(writer)

	got := style.Assistant("Assistant> ")
	if !strings.Contains(got, "Assistant> ") || !strings.Contains(got, ansiReset) {
		t.Fatalf("Assistant() = %q, want ANSI wrapped text", got)
	}
}

type fakeColorWriter struct {
	enabled bool
}

func (w fakeColorWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (w fakeColorWriter) ColorEnabled() bool {
	return w.enabled
}
