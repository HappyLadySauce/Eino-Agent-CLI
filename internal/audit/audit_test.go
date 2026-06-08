package audit

import (
	"context"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	got := Redact(`{"token":"abc","api_key=secret"}`)
	if strings.Contains(got, "secret") {
		t.Fatalf("Redact() = %q, want secret removed", got)
	}
}

func TestMemorySinkAppend(t *testing.T) {
	sink := NewMemorySink()
	record := Record{ID: NewID(), SessionID: "session-1"}
	if err := sink.Append(context.Background(), record); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	records := sink.Records()
	if len(records) != 1 || records[0].SessionID != "session-1" {
		t.Fatalf("Records() = %+v, want one record", records)
	}
}
