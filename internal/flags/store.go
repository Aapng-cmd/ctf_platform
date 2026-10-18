// Package flags stores Attack-Defense flags in Redis.
// A flag is written and read on every round and looked up on every submit, so it
// does not go through Postgres. Jeopardy challenge flags stay in Postgres: they
// are task content, not the per-round hot path.
package flags

import (
	"errors"

	"github.com/aapng-cmd/ctf_platform/internal/models"
	"time"
)

var ErrDuplicate = errors.New("flag already stored")

// Store is the Attack-Defense flag index. Implementations must be safe for
// concurrent use by the round engine and flag submission.
type Store interface {
	Save(fl *models.ADFlag) error
	ByValue(flag string) (models.ADFlag, bool)
	ByRoundTeamService(roundID, teamID, serviceID uint) (models.ADFlag, bool)
	ByRound(roundID uint) []models.ADFlag
	NeedsPut(roundID uint) []models.ADFlag
	SetPut(id uint, status, message string) error
	Unsettled(before time.Time, limit int) []models.ADFlag
	Settle(id uint, defended bool) error
	Defended() map[[2]uint]int
	Recent(limit int) []models.ADFlag
	DeleteService(serviceID uint)
	DeleteAll()
}
