package flags

import (
	"testing"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/alicebob/miniredis/v2"
)

func TestMemoryStore(t *testing.T) {
	exercise(t, NewMemory())
}

func TestRedisStore(t *testing.T) {
	srv := miniredis.RunT(t)
	st, err := Open(srv.Addr(), "", 0)
	if err != nil {
		t.Fatal(err)
	}
	exercise(t, st)
}

func exercise(t *testing.T, s Store) {
	t.Helper()
	now := time.Now().Truncate(time.Millisecond)
	live := models.ADFlag{
		RoundID: 1, ServiceID: 2, TeamID: 3, Flag: "FLAG{a}",
		Issued: now, Expires: now.Add(time.Hour),
	}
	if err := s.Save(&live); err != nil {
		t.Fatal(err)
	}
	if live.ID == 0 {
		t.Fatal("id was not assigned")
	}
	dup := models.ADFlag{Flag: "FLAG{a}", RoundID: 1, ServiceID: 2, TeamID: 9, Issued: now, Expires: now.Add(time.Hour)}
	if err := s.Save(&dup); err != ErrDuplicate {
		t.Fatalf("duplicate save returned %v", err)
	}

	got, ok := s.ByValue("FLAG{a}")
	if !ok || got.TeamID != 3 || got.ID != live.ID {
		t.Fatalf("by value = %+v %v", got, ok)
	}
	if err := s.SetPut(live.ID, models.CheckError, "checker unavailable: dial"); err != nil {
		t.Fatal(err)
	}
	got, ok = s.ByRoundTeamService(1, 3, 2)
	if !ok || got.PutStatus != models.CheckError {
		t.Fatalf("after put = %+v %v", got, ok)
	}
	if len(s.NeedsPut(1)) != 1 {
		t.Fatalf("needs put = %d", len(s.NeedsPut(1)))
	}

	old := models.ADFlag{
		RoundID: 1, ServiceID: 2, TeamID: 4, Flag: "FLAG{b}",
		Issued: now.Add(-2 * time.Hour), Expires: now.Add(-time.Minute), PutStatus: models.CheckOK,
	}
	if err := s.Save(&old); err != nil {
		t.Fatal(err)
	}
	pending := s.Unsettled(time.Now(), 10)
	if len(pending) != 1 || pending[0].Flag != "FLAG{b}" {
		t.Fatalf("unsettled = %+v", pending)
	}
	if err := s.Settle(old.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Settle(old.ID, true); err != nil {
		t.Fatal(err)
	}
	if n := s.Defended()[[2]uint{4, 2}]; n != 1 {
		t.Fatalf("defended count = %d", n)
	}
	if again := s.Unsettled(time.Now(), 10); len(again) != 0 {
		t.Fatalf("still unsettled: %+v", again)
	}
	recent := s.Recent(10)
	if len(recent) != 2 || recent[0].ID < recent[1].ID {
		t.Fatalf("recent = %+v", recent)
	}

	s.DeleteService(2)
	if _, ok := s.ByValue("FLAG{a}"); ok {
		t.Fatal("flag survived service delete")
	}
	if _, ok := s.ByValue("FLAG{b}"); ok {
		t.Fatal("defended flag survived service delete")
	}
	if len(s.Defended()) != 0 {
		t.Fatalf("defended left: %v", s.Defended())
	}

	keep := models.ADFlag{RoundID: 2, ServiceID: 7, TeamID: 1, Flag: "FLAG{c}", Issued: now, Expires: now.Add(time.Minute)}
	if err := s.Save(&keep); err != nil {
		t.Fatal(err)
	}
	s.DeleteAll()
	if _, ok := s.ByValue("FLAG{c}"); ok {
		t.Fatal("flag survived delete all")
	}
}
