package handlers

import (
	"testing"

	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/scoring"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAddTeamAdjustKeepsTheFinalSum(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:adjust?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ADAdjust{}); err != nil {
		t.Fatal(err)
	}
	for _, delta := range []int{10, 5, -3} {
		if err := addTeamAdjust(db, 1, delta); err != nil {
			t.Fatal(err)
		}
	}
	var row models.ADAdjust
	if err := db.Where("team_id = ?", 1).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Points != 12 {
		t.Fatalf("points = %d, want the sum 12", row.Points)
	}
	if got := (scoring.ADStanding{Score: 2, Adjust: row.Points}).ScoreLabel(); got != "14 (+ 12)" {
		t.Fatalf("label %s", got)
	}
	if err := addTeamAdjust(db, 1, -12); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("team_id = ?", 1).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Points != 0 {
		t.Fatalf("points = %d, want 0", row.Points)
	}
	if got := (scoring.ADStanding{Score: 2, Adjust: row.Points}).ScoreLabel(); got != "2" {
		t.Fatalf("zero sum label %s", got)
	}
}
