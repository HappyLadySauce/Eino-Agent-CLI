package security

import "testing"

func TestRateLimiterSessionLimit(t *testing.T) {
	limiter := NewRateLimiter()
	limit := RateLimitDescriptor{MaxCallsPerSession: 1}
	if got := limiter.CheckAndRecord("session-1", "read_file", limit); got != nil {
		t.Fatalf("first CheckAndRecord() = %+v, want nil", got)
	}
	got := limiter.CheckAndRecord("session-1", "read_file", limit)
	if got == nil || got.Status != ResultStatusRateLimited {
		t.Fatalf("second CheckAndRecord() = %+v, want rate_limited", got)
	}
}

func TestRateLimiterDeniedStreak(t *testing.T) {
	limiter := NewRateLimiter()
	if got := limiter.RecordDenied("session-1", "run_command"); got != 1 {
		t.Fatalf("RecordDenied() = %d, want 1", got)
	}
	if got := limiter.RecordDenied("session-1", "run_command"); got != 2 {
		t.Fatalf("RecordDenied() = %d, want 2", got)
	}
}
