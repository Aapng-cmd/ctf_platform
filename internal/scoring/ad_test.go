package scoring

import (
	"testing"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func adDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Team{}, &models.ADService{}, &models.ADInstance{}, &models.ADRound{}, &models.ADFlag{}, &models.ADCheck{}, &models.ADSteal{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestADDashboardScoring(t *testing.T) {
	db := adDB(t)
	now := time.Now()
	svc := models.ADService{Name: "bucket", Active: true,
		AttackReward: models.ADDefaultAttackReward, DefensePenalty: models.ADDefaultDefensePenalty,
		SLAReward: models.ADDefaultSLAReward, DownPenalty: models.ADDefaultDownPenalty}
	db.Create(&svc)
	alpha := models.Team{Name: "alpha"}
	bravo := models.Team{Name: "bravo"}
	idle := models.Team{Name: "idle"}
	db.Create(&alpha)
	db.Create(&bravo)
	db.Create(&idle)
	db.Create(&models.ADInstance{TeamID: alpha.ID, ServiceID: svc.ID, Host: "10.0.0.1", Port: 18101})
	db.Create(&models.ADInstance{TeamID: bravo.ID, ServiceID: svc.ID, Host: "10.0.0.2", Port: 18102})
	db.Create(&models.ADInstance{TeamID: idle.ID, ServiceID: svc.ID})
	r1 := models.ADRound{Number: 1, Started: now}
	r2 := models.ADRound{Number: 2, Started: now}
	db.Create(&r1)
	db.Create(&r2)
	for _, c := range []models.ADCheck{
		{RoundID: r1.ID, ServiceID: svc.ID, TeamID: alpha.ID, Status: models.CheckOK},
		{RoundID: r1.ID, ServiceID: svc.ID, TeamID: bravo.ID, Status: models.CheckOK},
		{RoundID: r1.ID, ServiceID: svc.ID, TeamID: idle.ID, Status: models.CheckDown, Message: models.ADNoHostMessage},
		{RoundID: r2.ID, ServiceID: svc.ID, TeamID: alpha.ID, Status: models.CheckOK},
		{RoundID: r2.ID, ServiceID: svc.ID, TeamID: bravo.ID, Status: models.CheckDown, Message: "connection refused"},
		{RoundID: r2.ID, ServiceID: svc.ID, TeamID: idle.ID, Status: models.CheckDown, Message: models.ADNoHostMessage},
	} {
		c.Date = now
		db.Create(&c)
	}
	db.Create(&models.ADSteal{FlagID: 1, AttackerTeamID: bravo.ID, VictimTeamID: alpha.ID, ServiceID: svc.ID, RoundID: r1.ID, Points: svc.AttackReward, Date: now})

	board := ADDashboard(db, map[[2]uint]int{{bravo.ID, svc.ID}: 1}, false)
	if len(board.Services) != 1 || len(board.Teams) != 3 {
		t.Fatalf("board = %+v", board)
	}
	by := map[string]ADStanding{}
	for _, st := range board.Teams {
		by[st.Name] = st
	}
	// Three teams play, so every failed round costs DownPenalty × 2 opponents.
	a, b, i := by["alpha"], by["bravo"], by["idle"]
	if a.SLA != 4 || a.Lost != 10 || a.Defense != 0 || a.Score != -6 {
		t.Fatalf("alpha = %+v", a)
	}
	if b.Attack != 10 || b.Defense != 5 || b.SLA != 2 || b.Down != 40 || b.Score != -23 {
		t.Fatalf("bravo = %+v", b)
	}
	if i.Down != 80 || i.Score != -80 || i.Services[0].Status != models.CheckNoHost || i.Services[0].DownRounds != 2 {
		t.Fatalf("idle = %+v", i)
	}
	if a.Services[0].Status != models.CheckOK || b.Services[0].Status != models.CheckDown {
		t.Fatalf("statuses alpha=%s bravo=%s", a.Services[0].Status, b.Services[0].Status)
	}
	if got := b.Services[0].History; len(got) != 2 || got[0] != models.CheckOK || got[1] != models.CheckDown {
		t.Fatalf("bravo history = %v", got)
	}
	if a.Services[0].Lost != 1 || b.Services[0].Captured != 1 || b.Services[0].Defended != 1 {
		t.Fatalf("flag counts alpha lost=%d bravo captured=%d defended=%d", a.Services[0].Lost, b.Services[0].Captured, b.Services[0].Defended)
	}
	if b.Down/2 <= a.Lost {
		t.Fatalf("a down round (%d) must cost more than a stolen flag (%d)", b.Down/2, a.Lost)
	}
}

func TestADDownPenaltyScalesWithOpponents(t *testing.T) {
	db := adDB(t)
	now := time.Now()
	svc := models.ADService{Name: "svc", Active: true, AttackReward: 10, DefensePenalty: 10, SLAReward: 2, DownPenalty: 20}
	db.Create(&svc)
	var down models.Team
	for i, name := range []string{"down", "b", "c", "d"} {
		tm := models.Team{Name: name}
		db.Create(&tm)
		db.Create(&models.ADInstance{TeamID: tm.ID, ServiceID: svc.ID, Host: "10.0.0.1", Port: 1000 + i})
		if name == "down" {
			down = tm
		}
	}
	r := models.ADRound{Number: 1, Started: now}
	db.Create(&r)
	db.Create(&models.ADCheck{RoundID: r.ID, ServiceID: svc.ID, TeamID: down.ID, Status: models.CheckMumble, Date: now})
	for _, st := range ADScoreboard(db, nil, false) {
		if st.Name != "down" {
			continue
		}
		robbedByAll := 3 * svc.DefensePenalty
		if st.Down != 60 || st.Down <= robbedByAll {
			t.Fatalf("down = %d, robbed by all = %d", st.Down, robbedByAll)
		}
		return
	}
	t.Fatal("team not found")
}
