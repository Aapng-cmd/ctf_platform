package scoring

import (
	"testing"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRevealedHintsComeOffTheChallenge(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:hints?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Team{}, &models.Challenge{}, &models.Solve{}, &models.Hint{}, &models.Unlock{}, &models.Award{}); err != nil {
		t.Fatal(err)
	}
	ada := models.User{Name: "ada", Email: "ada@x", Password: "x"}
	bob := models.User{Name: "bob", Email: "bob@x", Password: "x"}
	db.Create(&ada)
	db.Create(&bob)
	open := models.Challenge{Name: "open", Category: "Misc", Value: 100, State: models.StateVisible}
	hidden := models.Challenge{Name: "hidden", Category: "Misc", Value: 50, State: models.StateHidden}
	plain := models.Challenge{Name: "plain", Category: "Misc", Value: 10, State: models.StateVisible}
	db.Create(&open)
	db.Create(&hidden)
	db.Create(&plain)
	now := time.Now()
	db.Create(&models.Solve{ChallengeID: open.ID, UserID: ada.ID, Date: now})
	db.Create(&models.Solve{ChallengeID: hidden.ID, UserID: ada.ID, Date: now})
	db.Create(&models.Solve{ChallengeID: plain.ID, UserID: bob.ID, Date: now})
	cheap := models.Hint{ChallengeID: open.ID, Content: "a", Cost: 40}
	rich := models.Hint{ChallengeID: open.ID, Content: "b", Cost: 80}
	other := models.Hint{ChallengeID: open.ID, Content: "c", Cost: 100}
	db.Create(&cheap)
	db.Create(&rich)
	db.Create(&other)
	db.Create(&models.Unlock{UserID: ada.ID, Target: cheap.ID, Type: models.UnlockHint, Date: now})
	db.Create(&models.Unlock{UserID: ada.ID, Target: rich.ID, Type: models.UnlockHint, Date: now})
	db.Create(&models.Unlock{UserID: bob.ID, Target: other.ID, Type: models.UnlockHint, Date: now})

	board := UserScoreboard(db, nil, true)
	got := map[string]Standing{}
	for _, row := range board {
		got[row.Name] = row
	}
	if got["ada"].Score != 0 || got["ada"].Solves != 1 {
		t.Fatalf("ada = %+v", got["ada"])
	}
	if got["bob"].Score != 10 || got["bob"].Solves != 1 {
		t.Fatalf("bob = %+v", got["bob"])
	}
	if Awarded(100, 40) != 60 || Awarded(100, 100) != 0 || Awarded(100, 0) != 100 {
		t.Fatal("awarded")
	}
}
