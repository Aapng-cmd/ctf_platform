package store

import (
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/aapng-cmd/ctf_platform/internal/models"
	"gorm.io/gorm"
)

type Store struct {
	DB *gorm.DB
	mu sync.RWMutex
	kv map[string]string
}

func New(db *gorm.DB) *Store {
	s := &Store{DB: db, kv: map[string]string{}}
	s.Reload()
	return s
}

func (s *Store) Reload() {
	var rows []models.ConfigKV
	s.DB.Find(&rows)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kv = map[string]string{}
	for _, r := range rows {
		s.kv[r.Key] = r.Value
	}
}

func (s *Store) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.kv[key]
}

func (s *Store) GetDefault(key, fallback string) string {
	v := s.Get(key)
	if v == "" {
		return fallback
	}
	return v
}

func (s *Store) Bool(key string) bool {
	v := strings.ToLower(s.Get(key))
	return v == "1" || v == "true" || v == "yes"
}

func (s *Store) Int(key string, fallback int) int {
	v := s.Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func upsertConfig(tx *gorm.DB, key, value string) error {
	row := models.ConfigKV{Key: key}
	// The column is named key. Pass it as a field so the driver quotes it;
	// a raw `key` is MySQL syntax and Postgres rejects it.
	return tx.Where(&models.ConfigKV{Key: key}).Assign(models.ConfigKV{Value: value}).FirstOrCreate(&row).Error
}

func (s *Store) Set(key, value string) error {
	if err := upsertConfig(s.DB, key, value); err != nil {
		return err
	}
	s.mu.Lock()
	s.kv[key] = value
	s.mu.Unlock()
	return nil
}

func (s *Store) SetMany(pairs map[string]string) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		for k, v := range pairs {
			if err := upsertConfig(tx, k, v); err != nil {
				return err
			}
		}
		s.mu.Lock()
		for k, v := range pairs {
			s.kv[k] = v
		}
		s.mu.Unlock()
		return nil
	})
}

func (s *Store) SetupComplete() bool {
	return s.Get("setup") == "true"
}

func (s *Store) CTFName() string {
	return s.GetDefault("ctf_name", "CTFd")
}

func (s *Store) UserMode() string {
	if s.IsAD() {
		return models.ModeTeams
	}
	mode := s.GetDefault("user_mode", models.ModeUsers)
	if mode != models.ModeTeams {
		return models.ModeUsers
	}
	return models.ModeTeams
}

func (s *Store) GameMode() string {
	mode := s.GetDefault("game_mode", models.GameJeopardy)
	switch mode {
	case models.GameAttackDefense, models.GameMixed:
		return mode
	default:
		return models.GameJeopardy
	}
}

func (s *Store) IsAD() bool {
	m := s.GameMode()
	return m == models.GameAttackDefense || m == models.GameMixed
}

func (s *Store) IsJeopardy() bool {
	m := s.GameMode()
	return m == models.GameJeopardy || m == models.GameMixed
}

func (s *Store) ADPaused() bool {
	return s.Bool("ad_paused")
}

func (s *Store) ADRoundSeconds() int {
	n := s.Int("ad_round_seconds", 15)
	if n < 15 {
		return 15
	}
	return n
}

func (s *Store) ADFlagLifetime() int {
	n := s.Int("ad_flag_lifetime_rounds", 3)
	if n < 1 {
		return 1
	}
	return n
}

func (s *Store) ADFlagPrefix() string {
	return s.GetDefault("ad_flag_prefix", "FLAG{")
}

func (s *Store) ADRewards() models.ADRewards {
	base := models.DefaultADRewards()
	return models.ADRewards{
		Attack:  s.Int("ad_attack_reward", base.Attack),
		Defense: s.Int("ad_defense_reward", base.Defense),
		Lost:    s.Int("ad_lost_penalty", base.Lost),
		SLA:     s.Int("ad_sla_reward", base.SLA),
		Down:    s.Int("ad_down_penalty", base.Down),
	}
}

func (s *Store) ADPrepMinutes() int {
	n := s.Int("ad_prep_minutes", 0)
	if n < 0 {
		return 0
	}
	return n
}

func (s *Store) ADPrepStart() *time.Time { return s.ParseTime("ad_prep_start") }

// ADPrepEnd is the moment checkers and scoring may start. A missing start or a
// zero duration means there is no preparation window.
func (s *Store) ADPrepEnd() *time.Time {
	start := s.ADPrepStart()
	mins := s.ADPrepMinutes()
	if start == nil || mins <= 0 {
		return nil
	}
	end := start.Add(time.Duration(mins) * time.Minute)
	return &end
}

// Preparing is true from before the window until it ends, so checkers stay off
// until the configured start time plus the preparation length.
func (s *Store) Preparing() bool {
	end := s.ADPrepEnd()
	return end != nil && time.Now().Before(*end)
}

func (s *Store) PrepSecondsLeft() int {
	end := s.ADPrepEnd()
	if end == nil {
		return 0
	}
	left := time.Until(*end)
	if left <= 0 {
		return 0
	}
	return int((left + time.Second - 1) / time.Second)
}

func (s *Store) IsTeams() bool {
	return s.UserMode() == models.ModeTeams
}

// Timezone is the single clock for every participant. An empty or unknown
// value means UTC.
func (s *Store) Timezone() string {
	z := strings.TrimSpace(s.Get("timezone"))
	if z == "" {
		return "UTC"
	}
	if _, err := time.LoadLocation(z); err != nil {
		return "UTC"
	}
	return z
}

func (s *Store) Location() *time.Location {
	loc, err := time.LoadLocation(s.Timezone())
	if err != nil {
		return time.UTC
	}
	return loc
}

func (s *Store) ParseTime(key string) *time.Time {
	return parseClock(s.Get(key), s.Location())
}

// parseClock reads an absolute RFC3339 instant or a wall-clock value in loc.
func parseClock(v string, loc *time.Location) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if loc == nil {
		loc = time.UTC
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, v, loc); err == nil {
			return &t
		}
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		t := time.Unix(n, 0).UTC()
		return &t
	}
	return nil
}

func (s *Store) Start() *time.Time { return s.ParseTime("start") }
func (s *Store) End() *time.Time   { return s.ParseTime("end") }
func (s *Store) Freeze() *time.Time {
	if t := s.ParseTime("freeze"); t != nil {
		return t
	}
	return nil
}

func (s *Store) Started() bool {
	start := s.Start()
	return start == nil || !time.Now().Before(*start)
}

func (s *Store) Ended() bool {
	end := s.End()
	return end != nil && time.Now().After(*end)
}

func (s *Store) DuringCTF() bool {
	return s.Started() && !s.Ended()
}

func (s *Store) ChallengeVisibility() string {
	return s.GetDefault("challenge_visibility", models.VisibilityPrivate)
}
func (s *Store) ScoreVisibility() string {
	return s.GetDefault("score_visibility", models.VisibilityPublic)
}
func (s *Store) AccountVisibility() string {
	return s.GetDefault("account_visibility", models.VisibilityPublic)
}
func (s *Store) RegistrationVisibility() string {
	return s.GetDefault("registration_visibility", models.VisibilityPublic)
}

func (s *Store) MaxTeamSize() int {
	return s.Int("max_team_size", 0)
}
