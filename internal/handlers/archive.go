package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/ad"
	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/tasks"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (a *App) AdminADStart(c *gin.Context) {
	if a.Store.IsAD() {
		a.flash(c, "warning", "Attack-Defense is already running")
		c.Redirect(http.StatusFound, "/admin/ad")
		return
	}
	ev := models.ADEvent{Started: time.Now()}
	if err := a.DB.Create(&ev).Error; err != nil {
		a.flash(c, "danger", "Could not start Attack-Defense")
		c.Redirect(http.StatusFound, "/admin/ad")
		return
	}
	_ = a.Store.SetMany(map[string]string{
		"ad_running":  "true",
		"ad_paused":   "true",
		"ad_event_id": strconv.Itoa(int(ev.ID)),
	})
	a.closeOpenTeams(0)
	a.rememberCoeffs(ev.ID, 0)
	a.dropADCache()
	a.flash(c, "success", "Attack-Defense started")
	c.Redirect(http.StatusFound, "/admin/ad")
}

func (a *App) AdminADEventStop(c *gin.Context) {
	if !a.Store.IsAD() {
		a.flash(c, "warning", "Attack-Defense is not running")
		c.Redirect(http.StatusFound, "/admin/ad")
		return
	}
	if r := ad.CurrentRound(a.DB); r != nil {
		ad.CaptureRound(a.DB, a.Store, a.Flags, r.Number)
	}
	id := uint(a.Store.Int("ad_event_id", 0))
	now := time.Now()
	if id != 0 {
		a.DB.Model(&models.ADEvent{}).Where("id = ?", id).Update("ended", now)
		a.recordADResults(id)
		a.closeOpenTeams(id)
	}
	_ = a.Store.SetMany(map[string]string{
		"ad_running":  "false",
		"ad_paused":   "true",
		"ad_event_id": "0",
	})
	a.deleteCheckers()
	a.DB.Where("id > ?", 0).Delete(&models.ADCheck{})
	a.DB.Where("id > ?", 0).Delete(&models.ADSteal{})
	a.DB.Where("id > ?", 0).Delete(&models.ADRound{})
	a.DB.Where("id > ?", 0).Delete(&models.ADAdjust{})
	if a.Flags != nil {
		a.Flags.DeleteAll()
	}
	a.dropADCache()
	a.flash(c, "success", "Attack-Defense stopped")
	c.Redirect(http.StatusFound, "/archive")
}

// deleteCheckers stops checker containers and drops their rows. Service names
// stay, so the next event can attach a new checker. Nothing about a checker is
// written into the archive.
func (a *App) deleteCheckers() {
	var checkers []models.ADChecker
	a.DB.Find(&checkers)
	var services []models.ADService
	a.DB.Select("id", "compose_dir").Find(&services)
	dirs := map[string]struct{}{}
	for _, ch := range checkers {
		if checkerOverlay(ch.ComposeDir) {
			dirs[ch.ComposeDir] = struct{}{}
		}
	}
	for _, svc := range services {
		if checkerOverlay(svc.ComposeDir) {
			dirs[svc.ComposeDir] = struct{}{}
		}
	}
	var mu sync.Mutex
	var stopped []string
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for dir := range dirs {
		wg.Add(1)
		sem <- struct{}{}
		go func(dir string) {
			defer wg.Done()
			defer func() { <-sem }()
			if _, err := tasks.DownChecker(dir); err != nil {
				log.Printf("checker remove: %v", err)
				return
			}
			mu.Lock()
			stopped = append(stopped, dir)
			mu.Unlock()
		}(dir)
	}
	wg.Wait()
	if len(stopped) > 0 {
		a.DB.Model(&models.ADService{}).Where("compose_dir IN ?", stopped).Updates(map[string]any{
			"deploy_status": "stopped",
			"deploy_log":    "stopped because Attack-Defense ended",
		})
	}
	a.DB.Model(&models.ADService{}).Where("id > ?", 0).Updates(map[string]any{
		"checker_id": gorm.Expr("NULL"), "checker_url": "", "checker_service": "",
	})
	a.DB.Where("id > ?", 0).Delete(&models.ADChecker{})
}

func checkerOverlay(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, "ctfd-checker.compose.yml"))
	return err == nil && !st.IsDir()
}

func (a *App) rememberCoeffs(eventID uint, round int) {
	if a.Store == nil || eventID == 0 {
		return
	}
	ad.RecordCoeffs(a.DB, eventID, round, a.Store.ADRewards(), a.Store.ADRoundSeconds(), a.Store.ADFlagLifetime())
}

func (a *App) coeffRound() int {
	if r := ad.CurrentRound(a.DB); r != nil {
		return r.Number
	}
	return 0
}

// addTeamAdjust adds delta to the team's running admin total. The stored number
// is the sum of every grant and removal, and the SQL update does not drop a zero.
func addTeamAdjust(db *gorm.DB, teamID uint, delta int) error {
	res := db.Model(&models.ADAdjust{}).Where("team_id = ?", teamID).Update("points", gorm.Expr("points + ?", delta))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}
	err := db.Create(&models.ADAdjust{TeamID: teamID, Points: delta}).Error
	if err == nil {
		return nil
	}
	res = db.Model(&models.ADAdjust{}).Where("team_id = ?", teamID).Update("points", gorm.Expr("points + ?", delta))
	return res.Error
}

func (a *App) AdminADAdjustPOST(c *gin.Context) {
	if !a.Store.IsAD() {
		a.flash(c, "danger", "Attack-Defense is not running")
		c.Redirect(http.StatusFound, "/admin/ad")
		return
	}
	teamID, _ := strconv.Atoi(c.PostForm("team_id"))
	points, err := strconv.Atoi(strings.TrimSpace(c.PostForm("points")))
	if teamID <= 0 || err != nil || points == 0 {
		a.flash(c, "danger", "Choose a team and a non-zero number of points")
		c.Redirect(http.StatusFound, "/admin/ad")
		return
	}
	var team models.Team
	if a.DB.First(&team, teamID).Error != nil || team.Banned {
		a.flash(c, "danger", "Team not found")
		c.Redirect(http.StatusFound, "/admin/ad")
		return
	}
	if err := addTeamAdjust(a.DB, team.ID, points); err != nil {
		a.flash(c, "danger", "Could not save points")
		c.Redirect(http.StatusFound, "/admin/ad")
		return
	}
	a.dropADCache()
	a.flash(c, "success", "Points saved")
	c.Redirect(http.StatusFound, "/admin/ad")
}

func (a *App) ArchiveList(c *gin.Context) {
	var events []models.ADEvent
	a.DB.Where("ended IS NOT NULL").Order("started DESC").Find(&events)
	a.HTML(c, http.StatusOK, "archive.html", gin.H{
		"Title": "Archive", "Events": events, "Flashes": a.flashes(c),
	})
}

type archivePack struct {
	Frames []json.RawMessage `json:"frames"`
	Coeffs []models.ADCoeff  `json:"coeffs"`
}

func (a *App) ArchiveView(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ev models.ADEvent
	if a.DB.First(&ev, id).Error != nil || ev.Ended == nil {
		a.HTML(c, http.StatusNotFound, "error.html", gin.H{"Title": "Archive", "Message": "Record not found"})
		return
	}
	var shots []models.ADSnapshot
	a.DB.Where("event_id = ?", ev.ID).Order("round").Find(&shots)
	frames := make([]json.RawMessage, 0, len(shots))
	for _, s := range shots {
		if json.Valid([]byte(s.Board)) {
			frames = append(frames, json.RawMessage(ad.ScrubPublicBoard([]byte(s.Board))))
		}
	}
	coeffs := []models.ADCoeff{}
	a.DB.Where("event_id = ?", ev.ID).Order("id").Find(&coeffs)
	raw, err := json.Marshal(archivePack{Frames: frames, Coeffs: coeffs})
	if err != nil {
		raw = []byte(`{"frames":[],"coeffs":[]}`)
	}
	a.HTML(c, http.StatusOK, "archive_view.html", gin.H{
		"Title": "Archive", "Event": ev, "HasFrames": len(frames) > 0,
		"Pack":       template.JS(raw),
		"PageScript": "/static/js/dashboard.js?v=10",
		"Flashes":    a.flashes(c),
	})
}

func (a *App) AdminADArchiveDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.DB.Where("event_id = ?", id).Delete(&models.ADSnapshot{})
	a.DB.Where("event_id = ?", id).Delete(&models.ADCoeff{})
	a.DB.Delete(&models.ADEvent{}, id)
	a.flash(c, "success", "Archive record deleted")
	c.Redirect(http.StatusFound, "/archive")
}
