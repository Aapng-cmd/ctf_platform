package ad

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/flags"
	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/store"
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

func TestRoundFlipDoesNotWaitForCheckers(t *testing.T) {
	// cache=shared keeps one memory database across the pool. A plain
	// file::memory: name gives every connection an empty database.
	db, err := gorm.Open(sqlite.Open("file:roundflip?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Team{}, &models.ADService{}, &models.ADInstance{}, &models.ADRound{}, &models.ADFlag{}, &models.ADCheck{}, &models.ADSteal{}, &models.ConfigKV{}); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(entered) })
		<-release
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "OK"})
	}))
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		slow.Close()
	})

	st := store.New(db)
	if err := st.SetMany(map[string]string{
		"game_mode":        models.GameAttackDefense,
		"ad_round_seconds": "15",
		"ad_paused":        "false",
	}); err != nil {
		t.Fatal(err)
	}
	e := &Engine{
		DB: db, Store: st, Flags: flags.NewMemory(),
		http: &http.Client{Timeout: 5 * time.Second},
	}
	svc := models.ADService{Name: "svc", Active: true, Port: 18091, CheckerURL: slow.URL}
	if err := db.Create(&svc).Error; err != nil {
		t.Fatal(err)
	}
	team := models.Team{Name: "alpha", ADHost: "203.0.113.10"}
	if err := db.Create(&team).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ADInstance{TeamID: team.ID, ServiceID: svc.ID, Host: "203.0.113.10", Port: 18091}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ADRound{Number: 1, Started: time.Now().Add(-20 * time.Second)}).Error; err != nil {
		t.Fatal(err)
	}

	began := time.Now()
	e.tick()
	if elapsed := time.Since(began); elapsed > 400*time.Millisecond {
		t.Fatalf("round flip waited %s for the checker", elapsed)
	}
	var rounds []models.ADRound
	if err := db.Order("number").Find(&rounds).Error; err != nil {
		t.Fatal(err)
	}
	if len(rounds) != 2 || rounds[1].Number != 2 || rounds[0].Ended == nil {
		t.Fatalf("rounds = %+v", rounds)
	}
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("checker was never called")
	}
	close(release)
}
