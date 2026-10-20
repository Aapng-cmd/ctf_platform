package handlers

import (
	"testing"
	"time"
)

func TestZoneOffset(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	if got := zoneOffset("Europe/Moscow", now); got != "UTC+3" {
		t.Fatalf("Moscow = %s", got)
	}
	if got := zoneOffset("America/Chicago", now); got != "UTC-5" {
		t.Fatalf("Chicago = %s", got)
	}
	if got := zoneOffset("Asia/Kolkata", now); got != "UTC+5:30" {
		t.Fatalf("Kolkata = %s", got)
	}
	if got := zoneOffset("UTC", now); got != "" {
		t.Fatalf("UTC = %q", got)
	}
	choices := zoneChoices("Europe/Moscow")
	if choices[2].Ru != "Москва" || choices[2].Offset != "UTC+3" {
		t.Fatalf("choice = %+v", choices[2])
	}
}
