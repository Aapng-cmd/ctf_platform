package handlers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/config"
	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/store"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

func TestTemplatesParse(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "web", "templates", "setup.html")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("templates not found")
		}
		root = parent
	}
	t.Chdir(root)
	db, err := gorm.Open(sqlite.Open("file:templates?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ConfigKV{}); err != nil {
		t.Fatal(err)
	}
	if _, err := New(db, store.New(db), config.Config{}); err != nil {
		t.Fatal(err)
	}
}
