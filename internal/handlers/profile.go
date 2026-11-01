package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/avatar"
	"github.com/aapng-cmd/ctf_platform/internal/middleware"
	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/scoring"
	"github.com/gin-gonic/gin"
)

type profileSolve struct {
	Name     string
	Category string
	Points   int
	Blood    bool
}

type profileAD struct {
	EventID uint
	When    time.Time
	Place   int
	Field   int
	Points  int
}

type profileCat struct {
	Name    string
	Solved  int
	Total   int
	Percent int
}

func (a *App) renderProfile(c *gin.Context, viewer *models.User, user models.User) {
	solves, bloods := a.profileSolves(user.ID)
	results := a.profileAD(user)
	cats := a.profileCategories(user.ID)
	a.syncAchievement(user.ID)
	var got models.UserAchievement
	title := ""
	if a.DB.Preload("Achievement").Where("user_id = ?", user.ID).First(&got).Error == nil {
		title = got.Achievement.Title
	}
	own := viewer != nil && viewer.ID == user.ID
	initial := ""
	for _, r := range user.Name {
		initial = string(r)
		break
	}
	a.HTML(c, http.StatusOK, "user.html", gin.H{
		"Title": user.Name, "Profile": user, "Own": own, "Initial": initial, "Achievement": title,
		"Solves": solves, "Bloods": bloods, "AD": results, "ADCount": len(results),
		"Categories": cats, "Flashes": a.flashes(c),
	})
}

func (a *App) profileSolves(userID uint) ([]profileSolve, int) {
	var rows []models.Solve
	a.DB.Preload("Challenge").Where("user_id = ?", userID).Order("date").Find(&rows)
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ChallengeID)
	}
	first := map[uint]uint{}
	if len(ids) > 0 {
		var all []models.Solve
		a.DB.Select("challenge_id", "user_id", "date").Where("challenge_id IN ?", ids).Order("date, id").Find(&all)
		for _, row := range all {
			if _, ok := first[row.ChallengeID]; !ok {
				first[row.ChallengeID] = row.UserID
			}
		}
	}
	costs := scoring.HintCosts(a.DB, userID)
	out := make([]profileSolve, 0, len(rows))
	bloods := 0
	for _, row := range rows {
		blood := first[row.ChallengeID] == userID
		if blood {
			bloods++
		}
		points := 0
		if row.Challenge.State == models.StateVisible {
			points = scoring.Awarded(row.Challenge.Value, costs[row.ChallengeID])
		}
		out = append(out, profileSolve{
			Name: row.Challenge.Name, Category: models.CanonicalCategory(row.Challenge.Category),
			Points: points, Blood: blood,
		})
	}
	return out, bloods
}

func (a *App) profileCategories(userID uint) []profileCat {
	var challenges []models.Challenge
	a.DB.Where("state = ?", models.StateVisible).Find(&challenges)
	total := map[string]int{}
	for _, ch := range challenges {
		total[models.CanonicalCategory(ch.Category)]++
	}
	var solves []models.Solve
	a.DB.Preload("Challenge").Where("user_id = ?", userID).Find(&solves)
	done := map[string]int{}
	seen := map[uint]bool{}
	for _, row := range solves {
		if seen[row.ChallengeID] || row.Challenge.ID == 0 || row.Challenge.State != models.StateVisible {
			continue
		}
		seen[row.ChallengeID] = true
		done[models.CanonicalCategory(row.Challenge.Category)]++
	}
	out := make([]profileCat, 0, len(models.Categories))
	for _, c := range models.Categories {
		n := total[c.Name]
		pct := 0
		if n > 0 {
			pct = done[c.Name] * 100 / n
		}
		out = append(out, profileCat{Name: c.Name, Solved: done[c.Name], Total: n, Percent: pct})
	}
	return out
}

func (a *App) profileAD(user models.User) []profileAD {
	a.backfillADResults()
	var rows []models.ADResult
	a.DB.Preload("Event").Where("user_id = ?", user.ID).Find(&rows)
	out := make([]profileAD, 0, len(rows))
	for _, row := range rows {
		when := row.Event.Started
		out = append(out, profileAD{
			EventID: row.EventID, When: when, Place: row.Place, Field: row.Field, Points: row.Points,
		})
	}
	// Newest event first.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].When.After(out[i].When) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func (a *App) backfillADResults() {
	var events []models.ADEvent
	a.DB.Where("ended IS NOT NULL").Find(&events)
	for _, ev := range events {
		var n int64
		a.DB.Model(&models.ADResult{}).Where("event_id = ?", ev.ID).Count(&n)
		if n == 0 {
			a.recordADResults(ev.ID)
		}
	}
}

func (a *App) recordADResults(eventID uint) {
	if eventID == 0 {
		return
	}
	var shot models.ADSnapshot
	if a.DB.Where("event_id = ?", eventID).Order("round DESC").First(&shot).Error != nil || shot.Board == "" {
		return
	}
	var board struct {
		Teams []struct {
			AccountID uint   `json:"account_id"`
			Name      string `json:"name"`
			Score     int    `json:"score"`
			Adjust    int    `json:"adjust"`
			Place     int    `json:"pos"`
		} `json:"teams"`
	}
	if json.Unmarshal([]byte(shot.Board), &board) != nil {
		return
	}
	field := len(board.Teams)
	for _, team := range board.Teams {
		var members []models.User
		a.DB.Where("team_id = ?", team.AccountID).Find(&members)
		for _, member := range members {
			row := models.ADResult{
				EventID: eventID, UserID: member.ID, TeamName: team.Name,
				Place: team.Place, Field: field, Points: team.Score + team.Adjust,
			}
			var prev models.ADResult
			err := a.DB.Where("event_id = ? AND user_id = ?", eventID, member.ID).First(&prev).Error
			if err != nil {
				_ = a.DB.Create(&row).Error
				continue
			}
			_ = a.DB.Model(&prev).Updates(map[string]any{
				"team_name": row.TeamName, "place": row.Place, "field": row.Field, "points": row.Points,
			}).Error
		}
	}
}

func (a *App) syncChallengeSolvers(challengeID uint) {
	var ids []uint
	a.DB.Model(&models.Solve{}).Where("challenge_id = ?", challengeID).Distinct().Pluck("user_id", &ids)
	for _, id := range ids {
		a.syncAchievement(id)
	}
}

func (a *App) syncAchievement(userID uint) {
	if userID == 0 {
		return
	}
	cats := a.profileCategories(userID)
	var best *profileCat
	bestAt := -1
	for i := range cats {
		if cats[i].Solved == 0 {
			continue
		}
		if best == nil || cats[i].Solved > best.Solved || (cats[i].Solved == best.Solved && cats[i].Percent > best.Percent) {
			best = &cats[i]
			bestAt = i
		}
	}
	if best == nil {
		a.DB.Where("user_id = ?", userID).Delete(&models.UserAchievement{})
		return
	}
	var ach models.Achievement
	if a.DB.Where("key = ?", models.Categories[bestAt].Key).First(&ach).Error != nil {
		return
	}
	var row models.UserAchievement
	err := a.DB.Where("user_id = ?", userID).First(&row).Error
	if err != nil {
		_ = a.DB.Create(&models.UserAchievement{UserID: userID, AchievementID: ach.ID}).Error
		return
	}
	if row.AchievementID != ach.ID {
		_ = a.DB.Model(&row).Update("achievement_id", ach.ID).Error
	}
}

func (a *App) AvatarGET(c *gin.Context) {
	viewer := middleware.CurrentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 || !a.canSeeAccounts(viewer) {
		c.Status(http.StatusNotFound)
		return
	}
	var user models.User
	if a.DB.First(&user, id).Error != nil || user.Avatar == "" {
		c.Status(http.StatusNotFound)
		return
	}
	if (user.Hidden || user.Banned) && (viewer == nil || !viewer.IsAdmin()) && (viewer == nil || viewer.ID != user.ID) {
		c.Status(http.StatusNotFound)
		return
	}
	name := strconv.Itoa(id) + ".png"
	if user.Avatar != name {
		c.Status(http.StatusNotFound)
		return
	}
	path := filepath.Join(a.Cfg.DataDir, "avatars", name)
	if _, err := os.Stat(path); err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, max-age=3600")
	c.File(path)
}

func (a *App) saveAvatar(c *gin.Context, userID uint) error {
	fh, err := c.FormFile("avatar")
	if err != nil {
		return nil
	}
	if fh.Size <= 0 || fh.Size > avatar.MaxBytes {
		return errString("picture is too large")
	}
	f, err := fh.Open()
	if err != nil {
		return errPicture
	}
	defer f.Close()
	pngBytes, err := avatar.Decode(f)
	if err != nil {
		return err
	}
	dir := filepath.Join(a.Cfg.DataDir, "avatars")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := strconv.FormatUint(uint64(userID), 10) + ".png"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, pngBytes, 0o644); err != nil {
		return err
	}
	return a.DB.Model(&models.User{}).Where("id = ?", userID).Update("avatar", name).Error
}

var errPicture = errString("picture must be a png or jpg")

type errString string

func (e errString) Error() string { return string(e) }
