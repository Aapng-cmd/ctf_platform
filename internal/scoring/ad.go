package scoring

import (
	"sort"

	"github.com/aapng-cmd/ctf_platform/internal/models"
	"gorm.io/gorm"
)

const adHistoryRounds = 8

type ADServiceCell struct {
	ServiceID  uint     `json:"service_id"`
	Status     string   `json:"status"`
	Message    string   `json:"message,omitempty"`
	History    []string `json:"history"`
	Captured   int      `json:"captured"`
	Lost       int      `json:"lost"`
	Defended   int      `json:"defended"`
	OKRounds   int      `json:"ok_rounds"`
	DownRounds int      `json:"down_rounds"`
	Rounds     int      `json:"rounds"`
	Attack     int      `json:"attack"`
	Defense    int      `json:"defense"`
	LostPoints int      `json:"lost_points"`
	SLA        int      `json:"sla"`
	Down       int      `json:"down"`
	Score      int      `json:"score"`
}

type ADStanding struct {
	AccountID   uint            `json:"account_id"`
	Name        string          `json:"name"`
	Attack      int             `json:"attack"`
	Defense     int             `json:"defense"`
	SLA         int             `json:"sla"`
	Lost        int             `json:"lost"`
	Down        int             `json:"down"`
	Score       int             `json:"score"`
	Place       int             `json:"pos"`
	Affiliation string          `json:"affiliation"`
	Host        string          `json:"host"`
	Services    []ADServiceCell `json:"services"`
}

type ADBoardService struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Attack  int    `json:"attack_reward"`
	Lost    int    `json:"defense_penalty"`
	Defense int    `json:"defense_reward"`
	SLA     int    `json:"sla_reward"`
	Down    int    `json:"down_penalty"`
}

type ADBoard struct {
	Services []ADBoardService `json:"services"`
	Teams    []ADStanding     `json:"teams"`
}

func ADScoreboard(db *gorm.DB, defended map[[2]uint]int, includeHidden bool, rewards models.ADRewards) []ADStanding {
	return ADDashboard(db, defended, includeHidden, rewards).Teams
}

// ADDashboard scores every team per service from the event-wide rewards:
// attack = points stored on the steal, lost = Lost per own flag stolen,
// defense = Defense per own flag that survived its lifetime unstolen,
// SLA = SLA per OK round, down = Down × opponents per failed round.
// Every non-banned, non-hidden team plays, so a missing host is a failed round.
// defended maps [teamID, serviceID] to the number of flags Redis marked defended.
func ADDashboard(db *gorm.DB, defended map[[2]uint]int, includeHidden bool, rewards models.ADRewards) ADBoard {
	type key struct{ team, svc uint }

	var teams []models.Team
	q := db.Where("banned = ?", false)
	if !includeHidden {
		q = q.Where("hidden = ?", false)
	}
	q.Order("id").Find(&teams)

	var svcs []models.ADService
	db.Order("id").Find(&svcs)

	var playing int64
	db.Model(&models.Team{}).Where("banned = ? AND hidden = ?", false, false).Count(&playing)
	opponents := int(playing) - 1
	if opponents < 1 {
		opponents = 1
	}

	var insts []models.ADInstance
	db.Select("team_id", "service_id", "host").Find(&insts)
	hosted := map[key]bool{}
	for _, in := range insts {
		if in.Host != "" {
			hosted[key{in.TeamID, in.ServiceID}] = true
		}
	}

	var rounds []models.ADRound
	db.Order("number DESC").Limit(adHistoryRounds).Find(&rounds)
	slot := map[uint]int{}
	roundIDs := make([]uint, 0, len(rounds))
	for i, r := range rounds {
		slot[r.ID] = len(rounds) - 1 - i
		roundIDs = append(roundIDs, r.ID)
	}

	cells := map[key]*ADServiceCell{}
	cell := func(team, svc uint) *ADServiceCell {
		k := key{team, svc}
		if c := cells[k]; c != nil {
			return c
		}
		c := &ADServiceCell{ServiceID: svc, History: make([]string, len(rounds))}
		cells[k] = c
		return c
	}

	var steals []models.ADSteal
	db.Select("attacker_team_id", "victim_team_id", "service_id", "points").Find(&steals)
	for _, s := range steals {
		atk := cell(s.AttackerTeamID, s.ServiceID)
		atk.Captured++
		atk.Attack += s.Points
		vic := cell(s.VictimTeamID, s.ServiceID)
		vic.Lost++
		vic.LostPoints += rewards.Lost
	}

	type countAgg struct {
		TeamID    uint
		ServiceID uint
		Status    string
		N         int
	}
	for k, n := range defended {
		c := cell(k[0], k[1])
		c.Defended += n
		c.Defense += n * rewards.Defense
	}

	var aggs []countAgg
	db.Model(&models.ADCheck{}).Select("team_id, service_id, status, COUNT(*) AS n").Group("team_id, service_id, status").Scan(&aggs)
	for _, r := range aggs {
		c := cell(r.TeamID, r.ServiceID)
		c.Rounds += r.N
		if r.Status == models.CheckOK {
			c.OKRounds += r.N
			c.SLA += r.N * rewards.SLA
			continue
		}
		c.DownRounds += r.N
		c.Down += r.N * rewards.Down * opponents
	}

	if len(roundIDs) > 0 {
		var recent []models.ADCheck
		db.Select("round_id", "team_id", "service_id", "status").Where("round_id IN ?", roundIDs).Find(&recent)
		for _, ch := range recent {
			cell(ch.TeamID, ch.ServiceID).History[slot[ch.RoundID]] = ch.Status
		}
	}
	var latest []models.ADCheck
	db.Where("id IN (?)", db.Model(&models.ADCheck{}).Select("MAX(id)").Group("team_id, service_id")).Find(&latest)
	for _, ch := range latest {
		c := cell(ch.TeamID, ch.ServiceID)
		c.Status = ch.Status
		c.Message = ch.Message
	}

	board := ADBoard{Services: []ADBoardService{}, Teams: make([]ADStanding, 0, len(teams))}
	for _, s := range svcs {
		if s.Active {
			board.Services = append(board.Services, ADBoardService{
				ID: s.ID, Name: s.Name, Attack: rewards.Attack, Lost: rewards.Lost,
				Defense: rewards.Defense, SLA: rewards.SLA, Down: rewards.Down,
			})
		}
	}
	for _, t := range teams {
		st := ADStanding{AccountID: t.ID, Name: t.Name, Affiliation: t.Affiliation, Host: t.ADHost, Services: []ADServiceCell{}}
		for _, s := range svcs {
			cv := *cell(t.ID, s.ID)
			if !hosted[key{t.ID, s.ID}] {
				cv.Status = models.CheckNoHost
			}
			cv.Score = cv.Attack + cv.Defense + cv.SLA - cv.LostPoints - cv.Down
			st.Attack += cv.Attack
			st.Defense += cv.Defense
			st.SLA += cv.SLA
			st.Lost += cv.LostPoints
			st.Down += cv.Down
			if s.Active {
				st.Services = append(st.Services, cv)
			}
		}
		st.Score = st.Attack + st.Defense + st.SLA - st.Lost - st.Down
		board.Teams = append(board.Teams, st)
	}
	sort.Slice(board.Teams, func(i, j int) bool {
		a, b := board.Teams[i], board.Teams[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.Attack != b.Attack {
			return a.Attack > b.Attack
		}
		return a.Name < b.Name
	})
	for i := range board.Teams {
		board.Teams[i].Place = i + 1
	}
	return board
}
