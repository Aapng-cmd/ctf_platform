package ad

import (
	"encoding/json"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/flags"
	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/scoring"
	"github.com/aapng-cmd/ctf_platform/internal/store"
	"gorm.io/gorm"
)

type roundShot struct {
	Archive   bool                     `json:"archive"`
	Round     map[string]int           `json:"round"`
	Paused    bool                     `json:"paused"`
	During    bool                     `json:"during"`
	Preparing bool                     `json:"preparing"`
	Services  []scoring.ADBoardService `json:"services"`
	Teams     []scoring.ADStanding     `json:"teams"`
	Rewards   models.ADRewards         `json:"rewards"`
	FlagLife  int                      `json:"flag_life"`
}

// CaptureRound stores the public dashboard for one round of the live event.
// Team addresses are left out.
func CaptureRound(db *gorm.DB, st *store.Store, fl flags.Store, roundNumber int) {
	if db == nil || st == nil || roundNumber <= 0 {
		return
	}
	eventID := uint(st.Int("ad_event_id", 0))
	if eventID == 0 {
		return
	}
	defended := map[[2]uint]int{}
	if fl != nil {
		defended = fl.Defended()
	}
	rewards := st.ADRewards()
	seconds := st.ADRoundSeconds()
	life := st.ADFlagLifetime()
	RecordCoeffs(db, eventID, roundNumber, rewards, seconds, life)
	board := scoring.ADDashboard(db, defended, false, rewards)
	for i := range board.Teams {
		board.Teams[i].Host = ""
		for j := range board.Teams[i].Services {
			board.Teams[i].Services[j].Message = ""
		}
	}
	raw, err := json.Marshal(roundShot{
		Archive:  true,
		Round:    map[string]int{"number": roundNumber, "seconds_left": 0, "round_seconds": seconds},
		Services: board.Services,
		Teams:    board.Teams,
		Rewards:  rewards,
		FlagLife: life,
	})
	if err != nil {
		return
	}
	var row models.ADSnapshot
	err = db.Where("event_id = ? AND round = ?", eventID, roundNumber).First(&row).Error
	if err != nil {
		_ = db.Create(&models.ADSnapshot{EventID: eventID, Round: roundNumber, Board: string(raw), Taken: time.Now()}).Error
		return
	}
	_ = db.Model(&row).Updates(map[string]any{"board": string(raw), "taken": time.Now()}).Error
}

// ScrubPublicBoard drops team addresses and checker text from a stored dashboard.
// Older snapshots kept both, and the archive page is public.
func ScrubPublicBoard(raw []byte) []byte {
	if !json.Valid(raw) {
		return raw
	}
	var frame map[string]any
	if err := json.Unmarshal(raw, &frame); err != nil {
		return raw
	}
	teams, ok := frame["teams"].([]any)
	if !ok {
		return raw
	}
	for _, item := range teams {
		team, ok := item.(map[string]any)
		if !ok {
			continue
		}
		delete(team, "host")
		svcs, _ := team["services"].([]any)
		for _, svc := range svcs {
			cell, ok := svc.(map[string]any)
			if !ok {
				continue
			}
			delete(cell, "message")
		}
	}
	out, err := json.Marshal(frame)
	if err != nil {
		return raw
	}
	return out
}

// RecordCoeffs appends a coefficient row when the numbers differ from the last
// one saved for this event. Repeating the same settings does not add a row.
func RecordCoeffs(db *gorm.DB, eventID uint, round int, rewards models.ADRewards, roundSeconds, flagLife int) {
	if db == nil || eventID == 0 {
		return
	}
	if round < 0 {
		round = 0
	}
	if roundSeconds < 15 {
		roundSeconds = 15
	}
	if flagLife < 1 {
		flagLife = 1
	}
	var last models.ADCoeff
	err := db.Where("event_id = ?", eventID).Order("id DESC").First(&last).Error
	if err == nil && last.Attack == rewards.Attack && last.Defense == rewards.Defense && last.Lost == rewards.Lost && last.SLA == rewards.SLA && last.Down == rewards.Down && last.RoundSeconds == roundSeconds && last.FlagLife == flagLife {
		return
	}
	_ = db.Create(&models.ADCoeff{
		EventID: eventID, Round: round,
		Attack: rewards.Attack, Defense: rewards.Defense, Lost: rewards.Lost, SLA: rewards.SLA, Down: rewards.Down,
		RoundSeconds: roundSeconds, FlagLife: flagLife, At: time.Now(),
	}).Error
}
