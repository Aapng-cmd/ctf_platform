package handlers

import (
	"testing"
	"time"
)

func TestNextPrepStart(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	if got := nextPrepStart(0, "", 10, now); got != now.Format(time.RFC3339) {
		t.Fatalf("new window = %q", got)
	}
	kept := "2026-10-30T11:00:00Z"
	if got := nextPrepStart(10, kept, 10, now); got != kept {
		t.Fatalf("same duration changed the start to %q", got)
	}
	if got := nextPrepStart(10, kept, 15, now); got != now.Format(time.RFC3339) {
		t.Fatalf("changed duration = %q", got)
	}
	if got := nextPrepStart(10, kept, 0, now); got != "" {
		t.Fatalf("zero duration = %q", got)
	}
}
