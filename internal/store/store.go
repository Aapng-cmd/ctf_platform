package store

import (
	"strconv"
	"strings"
	"sync"
	"time"

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

func (s *Store) Set(key, value string) error {
	row := models.ConfigKV{Key: key, Value: value}
	if err := s.DB.Where("`key` = ?", key).Assign(models.ConfigKV{Value: value}).FirstOrCreate(&row).Error; err != nil {
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
			row := models.ConfigKV{Key: k, Value: v}
			if err := tx.Where("`key` = ?", k).Assign(models.ConfigKV{Value: v}).FirstOrCreate(&row).Error; err != nil {
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
	n := s.Int("ad_round_seconds", 60)
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

func (s *Store) IsTeams() bool {
	return s.UserMode() == models.ModeTeams
}

func (s *Store) ParseTime(key string) *time.Time {
	v := s.Get(key)
	if v == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, v); err == nil {
			return &t
		}
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		t := time.Unix(n, 0)
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
