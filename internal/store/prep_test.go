package store

import (
	"testing"
	"time"
)

func TestPreparationWindow(t *testing.T) {
	start := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)
	s := &Store{kv: map[string]string{
		"ad_prep_start":   start.Format(time.RFC3339),
		"ad_prep_minutes": "10",
	}}
	if !s.Preparing() {
		t.Fatal("expected the preparation window to be open")
	}
	left := s.PrepSecondsLeft()
	if left < 500 || left > 540 {
		t.Fatalf("seconds left = %d", left)
	}
	s.kv["ad_prep_minutes"] = "0"
	if s.Preparing() || s.PrepSecondsLeft() != 0 {
		t.Fatal("a zero duration is not a preparation window")
	}
	s.kv["ad_prep_minutes"] = "10"
	s.kv["ad_prep_start"] = time.Now().Add(-20 * time.Minute).UTC().Format(time.RFC3339)
	if s.Preparing() {
		t.Fatal("the window should already be over")
	}
}

func TestNaiveTimeUsesTimezone(t *testing.T) {
	s := &Store{kv: map[string]string{
		"timezone": "America/Chicago",
		"start":    "2026-10-18T09:00",
	}}
	got := s.Start()
	if got == nil {
		t.Fatal("start was not parsed")
	}
	if got.UTC().Format(time.RFC3339) != "2026-10-18T14:00:00Z" {
		t.Fatalf("start = %s", got.UTC().Format(time.RFC3339))
	}
	if s.Location().String() != "America/Chicago" {
		t.Fatalf("location = %s", s.Location())
	}
	s.kv["timezone"] = "Not/AZone"
	if s.Timezone() != "UTC" {
		t.Fatalf("bad zone fell back to %s", s.Timezone())
	}
}
