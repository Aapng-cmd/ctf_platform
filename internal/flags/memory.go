package flags

import (
	"sort"
	"sync"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/models"
)

// Memory is an in-process Store used by tests.
type Memory struct {
	mu    sync.Mutex
	seq   uint
	byID  map[uint]models.ADFlag
	byVal map[string]uint
	def   map[[2]uint]int
}

func NewMemory() *Memory {
	return &Memory{
		byID:  map[uint]models.ADFlag{},
		byVal: map[string]uint{},
		def:   map[[2]uint]int{},
	}
}

func (m *Memory) Save(fl *models.ADFlag) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byVal[fl.Flag]; ok {
		return ErrDuplicate
	}
	m.seq++
	fl.ID = m.seq
	m.byID[fl.ID] = *fl
	m.byVal[fl.Flag] = fl.ID
	return nil
}

func (m *Memory) ByValue(flag string) (models.ADFlag, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byVal[flag]
	if !ok {
		return models.ADFlag{}, false
	}
	return m.byID[id], true
}

func (m *Memory) ByRoundTeamService(roundID, teamID, serviceID uint) (models.ADFlag, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, fl := range m.byID {
		if fl.RoundID == roundID && fl.TeamID == teamID && fl.ServiceID == serviceID {
			return fl, true
		}
	}
	return models.ADFlag{}, false
}

func (m *Memory) ByRound(roundID uint) []models.ADFlag {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.ADFlag
	for _, fl := range m.byID {
		if fl.RoundID == roundID {
			out = append(out, fl)
		}
	}
	return out
}

func (m *Memory) NeedsPut(roundID uint) []models.ADFlag {
	var out []models.ADFlag
	for _, fl := range m.ByRound(roundID) {
		if fl.PutMessage == models.ADNoHostMessage || fl.PutStatus == models.CheckError {
			out = append(out, fl)
		}
	}
	return out
}

func (m *Memory) SetPut(id uint, status, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	fl, ok := m.byID[id]
	if !ok {
		return nil
	}
	fl.PutStatus = status
	fl.PutMessage = message
	m.byID[id] = fl
	return nil
}

func (m *Memory) Unsettled(before time.Time, limit int) []models.ADFlag {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.ADFlag
	for _, fl := range m.byID {
		if !fl.Settled && fl.Expires.Before(before) {
			out = append(out, fl)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (m *Memory) Settle(id uint, defended bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	fl, ok := m.byID[id]
	if !ok || fl.Settled {
		return nil
	}
	fl.Settled = true
	fl.Defended = defended
	m.byID[id] = fl
	if defended {
		m.def[[2]uint{fl.TeamID, fl.ServiceID}]++
	}
	return nil
}

func (m *Memory) Defended() map[[2]uint]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[[2]uint]int, len(m.def))
	for k, n := range m.def {
		if n > 0 {
			out[k] = n
		}
	}
	return out
}

func (m *Memory) Recent(limit int) []models.ADFlag {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]models.ADFlag, 0, len(m.byID))
	for _, fl := range m.byID {
		out = append(out, fl)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (m *Memory) dropLocked(fl models.ADFlag) {
	delete(m.byID, fl.ID)
	delete(m.byVal, fl.Flag)
	if fl.Defended {
		k := [2]uint{fl.TeamID, fl.ServiceID}
		m.def[k]--
		if m.def[k] <= 0 {
			delete(m.def, k)
		}
	}
}

func (m *Memory) DeleteService(serviceID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var gone []models.ADFlag
	for _, fl := range m.byID {
		if fl.ServiceID == serviceID {
			gone = append(gone, fl)
		}
	}
	for _, fl := range gone {
		m.dropLocked(fl)
	}
}

func (m *Memory) DeleteAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID = map[uint]models.ADFlag{}
	m.byVal = map[string]uint{}
	m.def = map[[2]uint]int{}
}
