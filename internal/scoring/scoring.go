package scoring

import (
	"math"
	"sort"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/store"
	"gorm.io/gorm"
)

func DynamicValue(ch models.Challenge, solves int) int {
	if ch.Type != models.ChallengeDynamic {
		return ch.Value
	}
	if ch.Decay <= 0 {
		return ch.Value
	}
	n := solves
	if n > 0 {
		n--
	}
	var value float64
	switch ch.Function {
	case "logarithmic":
		if n == 0 {
			value = float64(ch.Initial)
		} else {
			value = ((float64(ch.Minimum)-float64(ch.Initial))/math.Pow(float64(ch.Decay), 2))*math.Pow(math.Log(float64(n)+1), 2) + float64(ch.Initial)
		}
	default:
		value = ((float64(ch.Minimum)-float64(ch.Initial))/math.Pow(float64(ch.Decay), 2))*math.Pow(float64(n), 2) + float64(ch.Initial)
	}
	out := int(math.Ceil(value))
	if out < ch.Minimum {
		out = ch.Minimum
	}
	return out
}

func RecalcChallenge(db *gorm.DB, ch *models.Challenge) error {
	if ch.Type != models.ChallengeDynamic {
		return nil
	}
	var count int64
	db.Model(&models.Solve{}).Where("challenge_id = ?", ch.ID).Count(&count)
	ch.Value = DynamicValue(*ch, int(count))
	return db.Model(ch).Update("value", ch.Value).Error
}

type Standing struct {
	AccountID   uint      `json:"account_id"`
	Name        string    `json:"name"`
	Score       int       `json:"score"`
	Place       int       `json:"pos"`
	Solves      int       `json:"solves"`
	LastSolve   time.Time `json:"last_solve"`
	Affiliation string    `json:"affiliation"`
}

type accountAgg struct {
	id          uint
	name        string
	affiliation string
	score       int
	solves      int
	last        time.Time
}

func Scoreboard(db *gorm.DB, cfg *store.Store, freeze *time.Time, includeHidden bool) []Standing {
	type solveRow struct {
		ChallengeID uint
		UserID      uint
		TeamID      *uint
		Date        time.Time
		Value       int
		UserHidden  bool
		UserBanned  bool
		UserName    string
		UserAff     string
		TeamHidden  bool
		TeamBanned  bool
		TeamName    string
		TeamAff     string
	}

	q := db.Table("solves").
		Select("solves.challenge_id, solves.user_id, solves.team_id, solves.date, challenges.value, users.hidden as user_hidden, users.banned as user_banned, users.name as user_name, users.affiliation as user_aff, teams.hidden as team_hidden, teams.banned as team_banned, teams.name as team_name, teams.affiliation as team_aff").
		Joins("JOIN challenges ON challenges.id = solves.challenge_id").
		Joins("JOIN users ON users.id = solves.user_id").
		Joins("LEFT JOIN teams ON teams.id = solves.team_id")
	if freeze != nil {
		q = q.Where("solves.date <= ?", *freeze)
	}
	var rows []solveRow
	q.Scan(&rows)

	teams := cfg.IsTeams()
	aggs := map[uint]*accountAgg{}
	for _, r := range rows {
		var id uint
		var name, aff string
		hidden, banned := r.UserHidden, r.UserBanned
		if teams {
			if r.TeamID == nil {
				continue
			}
			id = *r.TeamID
			name, aff = r.TeamName, r.TeamAff
			hidden, banned = r.TeamHidden, r.TeamBanned
		} else {
			id = r.UserID
			name, aff = r.UserName, r.UserAff
		}
		if banned {
			continue
		}
		if hidden && !includeHidden {
			continue
		}
		a := aggs[id]
		if a == nil {
			a = &accountAgg{id: id, name: name, affiliation: aff}
			aggs[id] = a
		}
		a.score += r.Value
		a.solves++
		if r.Date.After(a.last) {
			a.last = r.Date
		}
	}

	type awardRow struct {
		UserID     uint
		TeamID     *uint
		Value      int
		Date       time.Time
		UserHidden bool
		UserBanned bool
		UserName   string
		UserAff    string
		TeamHidden bool
		TeamBanned bool
		TeamName   string
		TeamAff    string
	}
	aq := db.Table("awards").
		Select("awards.user_id, awards.team_id, awards.value, awards.date, users.hidden as user_hidden, users.banned as user_banned, users.name as user_name, users.affiliation as user_aff, teams.hidden as team_hidden, teams.banned as team_banned, teams.name as team_name, teams.affiliation as team_aff").
		Joins("JOIN users ON users.id = awards.user_id").
		Joins("LEFT JOIN teams ON teams.id = awards.team_id")
	if freeze != nil {
		aq = aq.Where("awards.date <= ?", freeze)
	}
	var awards []awardRow
	aq.Scan(&awards)
	for _, r := range awards {
		var id uint
		var name, aff string
		hidden, banned := r.UserHidden, r.UserBanned
		if teams {
			if r.TeamID == nil {
				continue
			}
			id = *r.TeamID
			name, aff = r.TeamName, r.TeamAff
			hidden, banned = r.TeamHidden, r.TeamBanned
		} else {
			id = r.UserID
			name, aff = r.UserName, r.UserAff
		}
		if banned || (hidden && !includeHidden) {
			continue
		}
		a := aggs[id]
		if a == nil {
			a = &accountAgg{id: id, name: name, affiliation: aff}
			aggs[id] = a
		}
		a.score += r.Value
		if r.Date.After(a.last) {
			a.last = r.Date
		}
	}

	type unlockRow struct {
		UserID uint
		TeamID *uint
		Cost   int
	}
	var unlocks []unlockRow
	db.Table("unlocks").
		Select("unlocks.user_id, unlocks.team_id, hints.cost").
		Joins("JOIN hints ON hints.id = unlocks.target").
		Where("unlocks.type = ?", models.UnlockHint).
		Scan(&unlocks)
	for _, r := range unlocks {
		id := r.UserID
		if teams {
			if r.TeamID == nil {
				continue
			}
			id = *r.TeamID
		}
		if a := aggs[id]; a != nil {
			a.score -= r.Cost
		}
	}

	out := make([]Standing, 0, len(aggs))
	for _, a := range aggs {
		out = append(out, Standing{
			AccountID:   a.id,
			Name:        a.name,
			Score:       a.score,
			Solves:      a.solves,
			LastSolve:   a.last,
			Affiliation: a.affiliation,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if !out[i].LastSolve.Equal(out[j].LastSolve) {
			if out[i].LastSolve.IsZero() {
				return false
			}
			if out[j].LastSolve.IsZero() {
				return true
			}
			return out[i].LastSolve.Before(out[j].LastSolve)
		}
		return out[i].Name < out[j].Name
	})
	for i := range out {
		out[i].Place = i + 1
	}
	return out
}

func AccountScore(db *gorm.DB, cfg *store.Store, userID uint, teamID *uint, freeze *time.Time) int {
	standings := Scoreboard(db, cfg, freeze, true)
	id := userID
	if cfg.IsTeams() && teamID != nil {
		id = *teamID
	}
	for _, s := range standings {
		if s.AccountID == id {
			return s.Score
		}
	}
	return 0
}

func AccountPlace(standings []Standing, id uint) int {
	for _, s := range standings {
		if s.AccountID == id {
			return s.Place
		}
	}
	return 0
}

func SolvedIDs(db *gorm.DB, userID uint, teamID *uint, teams bool) map[uint]bool {
	q := db.Model(&models.Solve{})
	if teams && teamID != nil {
		q = q.Where("team_id = ?", *teamID)
	} else {
		q = q.Where("user_id = ?", userID)
	}
	var ids []uint
	q.Pluck("challenge_id", &ids)
	out := map[uint]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func UnlockedHintIDs(db *gorm.DB, userID uint, teamID *uint, teams bool) map[uint]bool {
	q := db.Model(&models.Unlock{}).Where("type = ?", models.UnlockHint)
	if teams && teamID != nil {
		q = q.Where("team_id = ?", *teamID)
	} else {
		q = q.Where("user_id = ?", userID)
	}
	var ids []uint
	q.Pluck("target", &ids)
	out := map[uint]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func FirstBlood(db *gorm.DB, challengeID uint) *models.Solve {
	var s models.Solve
	err := db.Preload("User").Where("challenge_id = ?", challengeID).Order("date ASC").First(&s).Error
	if err != nil {
		return nil
	}
	return &s
}
