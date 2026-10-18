package ad

import (
	"testing"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/flags"
	"github.com/aapng-cmd/ctf_platform/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func engineDB(t *testing.T) *gorm.DB {
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

func TestSettleFlagsDefense(t *testing.T) {
	db := engineDB(t)
	e := &Engine{DB: db, Flags: flags.NewMemory()}
	svc := models.ADService{Name: "svc", Active: true}
	db.Create(&svc)
	team := models.Team{Name: "t"}
	other := models.Team{Name: "o"}
	db.Create(&team)
	db.Create(&other)

	start := time.Now().Add(-10 * time.Minute)
	var rounds []models.ADRound
	for i := 0; i < 6; i++ {
		r := models.ADRound{Number: i + 1, Started: start.Add(time.Duration(i) * time.Minute)}
		db.Create(&r)
		rounds = append(rounds, r)
	}
	check := func(r models.ADRound, status string) {
		db.Create(&models.ADCheck{RoundID: r.ID, ServiceID: svc.ID, TeamID: team.ID, Status: status, Date: r.Started.Add(time.Second)})
	}
	flag := func(name string, r models.ADRound, put string, life time.Duration) models.ADFlag {
		fl := models.ADFlag{RoundID: r.ID, ServiceID: svc.ID, TeamID: team.ID, Flag: name,
			Issued: r.Started, Expires: r.Started.Add(life), PutStatus: put}
		if err := e.Flags.Save(&fl); err != nil {
			t.Fatal(err)
		}
		return fl
	}

	check(rounds[0], models.CheckOK)
	check(rounds[1], models.CheckOK)
	check(rounds[2], models.CheckDown)
	check(rounds[3], models.CheckOK)
	check(rounds[4], models.CheckMumble)

	life := 90 * time.Second
	kept := flag("kept", rounds[0], models.CheckOK, life)
	stolen := flag("stolen", rounds[0], models.CheckOK, life)
	db.Create(&models.ADSteal{FlagID: stolen.ID, AttackerTeamID: other.ID, VictimTeamID: team.ID, ServiceID: svc.ID, RoundID: rounds[0].ID})
	downLater := flag("down-later", rounds[1], models.CheckOK, life)
	unseen := flag("unseen", rounds[4], models.CheckOK, 10*time.Second)
	putFailed := flag("put-failed", rounds[3], models.CheckDown, 10*time.Second)
	alive := flag("alive", rounds[5], models.CheckOK, time.Hour)

	e.settleFlags()

	want := map[uint][2]bool{
		kept.ID:      {true, true},
		stolen.ID:    {true, false},
		downLater.ID: {true, false},
		unseen.ID:    {true, false},
		putFailed.ID: {true, false},
		alive.ID:     {false, false},
	}
	for _, fl := range e.Flags.Recent(20) {
		w := want[fl.ID]
		if fl.Settled != w[0] || fl.Defended != w[1] {
			t.Errorf("%s: settled=%v defended=%v, want %v", fl.Flag, fl.Settled, fl.Defended, w)
		}
	}
}

func TestCheckWithoutHostIsDown(t *testing.T) {
	db := engineDB(t)
	e := &Engine{DB: db}
	svc := models.ADService{Name: "svc", Active: true}
	db.Create(&svc)
	team := models.Team{Name: "t"}
	db.Create(&team)
	inst := models.ADInstance{TeamID: team.ID, ServiceID: svc.ID}
	db.Create(&inst)
	r := models.ADRound{Number: 1, Started: time.Now()}
	db.Create(&r)
	db.Preload("Team").Preload("Service").First(&inst, inst.ID)

	e.checkOne(&r, inst)
	var ch models.ADCheck
	if err := db.Where("round_id = ? AND team_id = ?", r.ID, team.ID).First(&ch).Error; err != nil {
		t.Fatalf("no check row: %v", err)
	}
	if ch.Status != models.CheckDown || ch.Message != models.ADNoHostMessage {
		t.Fatalf("check = %s %q", ch.Status, ch.Message)
	}
}
