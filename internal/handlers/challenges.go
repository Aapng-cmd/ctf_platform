package handlers

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/middleware"
	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/scoring"
	"github.com/aapng-cmd/ctf_platform/internal/webutil"
	"github.com/gin-gonic/gin"
)

func (a *App) ChallengesPage(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.canSeeChallenges(u) {
		a.HTML(c, http.StatusForbidden, "error.html", gin.H{"Title": "Challenges", "Message": "Challenge visibility is restricted."})
		return
	}
	if !a.Store.Started() && (u == nil || !u.IsAdmin()) {
		a.HTML(c, http.StatusForbidden, "error.html", gin.H{"Title": "Challenges", "Message": "CTF has not started yet."})
		return
	}
	challenges := a.visibleChallenges(u)
	solved := map[uint]bool{}
	if u != nil {
		solved = scoring.SolvedIDs(a.DB, u.ID, u.TeamID, a.Store.IsTeams())
	}
	byCat := map[string][]models.Challenge{}
	var cats []string
	seen := map[string]bool{}
	for _, ch := range challenges {
		if !seen[ch.Category] {
			cats = append(cats, ch.Category)
			seen[ch.Category] = true
		}
		byCat[ch.Category] = append(byCat[ch.Category], ch)
	}
	a.HTML(c, http.StatusOK, "challenges.html", gin.H{
		"Title": "Challenges", "Categories": cats, "ByCategory": byCat, "Solved": solved, "Flashes": a.flashes(c),
	})
}

func (a *App) visibleChallenges(u *models.User) []models.Challenge {
	q := a.DB.Preload("Tags").Order("category, value, id")
	if u == nil || !u.IsAdmin() {
		q = q.Where("state = ?", models.StateVisible)
	}
	var all []models.Challenge
	q.Find(&all)
	if u == nil {
		return all
	}
	solved := scoring.SolvedIDs(a.DB, u.ID, u.TeamID, a.Store.IsTeams())
	var out []models.Challenge
	for _, ch := range all {
		if a.canAccessChallenge(ch, solved, u.IsAdmin()) {
			out = append(out, ch)
		}
	}
	return out
}

func (a *App) canAccessChallenge(ch models.Challenge, solved map[uint]bool, admin bool) bool {
	if admin {
		return true
	}
	if ch.State != models.StateVisible {
		return false
	}
	for _, id := range ch.RequiredIDs() {
		if !solved[id] {
			return false
		}
	}
	return true
}

func (a *App) ChallengeJSON(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.canSeeChallenges(u) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"Forbidden"}})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var ch models.Challenge
	if err := a.DB.Preload("Hints").Preload("Files").Preload("Tags").First(&ch, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "errors": []string{"Challenge not found"}})
		return
	}
	solved := map[uint]bool{}
	unlocked := map[uint]bool{}
	isSolved := false
	attempts := 0
	if u != nil {
		solved = scoring.SolvedIDs(a.DB, u.ID, u.TeamID, a.Store.IsTeams())
		unlocked = scoring.UnlockedHintIDs(a.DB, u.ID, u.TeamID, a.Store.IsTeams())
		isSolved = solved[ch.ID]
		q := a.DB.Model(&models.Submission{}).Where("challenge_id = ? AND type = ?", ch.ID, models.SubmissionIncorrect)
		if a.Store.IsTeams() && u.TeamID != nil {
			q = q.Where("team_id = ?", *u.TeamID)
		} else {
			q = q.Where("user_id = ?", u.ID)
		}
		var n int64
		q.Count(&n)
		attempts = int(n)
	}
	if !a.canAccessChallenge(ch, solved, u != nil && u.IsAdmin()) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "errors": []string{"Challenge not found"}})
		return
	}
	var solves int64
	a.DB.Model(&models.Solve{}).Where("challenge_id = ?", ch.ID).Count(&solves)
	fb := scoring.FirstBlood(a.DB, ch.ID)
	fbName := ""
	if fb != nil {
		fbName = fb.User.Name
	}
	hints := []gin.H{}
	for _, h := range ch.Hints {
		item := gin.H{"id": h.ID, "title": h.Title, "cost": h.Cost, "unlocked": unlocked[h.ID] || (u != nil && u.IsAdmin())}
		if unlocked[h.ID] || (u != nil && u.IsAdmin()) {
			item["content"] = h.Content
		}
		hints = append(hints, item)
	}
	files := []gin.H{}
	for _, f := range ch.Files {
		files = append(files, gin.H{"id": f.ID, "name": f.Name, "location": "/files/" + strconv.Itoa(int(f.ID))})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id": ch.ID, "name": ch.Name, "value": ch.Value,
			"description": string(webutil.Markdown(ch.Description)),
			"category":    ch.Category, "attribution": ch.Attribution, "connection_info": ch.ConnectionInfo,
			"max_attempts": ch.MaxAttempts, "attempts": attempts, "solves": solves, "first_blood": fbName,
			"solved_by_me": isSolved, "type": ch.Type, "state": ch.State,
			"hints": hints, "files": files, "tags": ch.Tags,
		},
	})
}

func (a *App) AttemptFlag(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "errors": []string{"Authentication required"}})
		return
	}
	if u.Banned {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"You have been banned"}})
		return
	}
	if a.Store.IsTeams() && u.TeamID == nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"You must join a team first"}})
		return
	}
	if !a.Store.DuringCTF() && !u.IsAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"CTF is not active"}})
		return
	}
	if !a.Limiter.Allow("attempt:" + strconv.Itoa(int(u.ID))) {
		c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "errors": []string{"You're submitting flags too fast"}})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Submission string `json:"submission"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		body.Submission = c.PostForm("submission")
	}
	provided := strings.TrimSpace(body.Submission)
	var ch models.Challenge
	if err := a.DB.Preload("Flags").First(&ch, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "errors": []string{"Challenge not found"}})
		return
	}
	solved := scoring.SolvedIDs(a.DB, u.ID, u.TeamID, a.Store.IsTeams())
	if !a.canAccessChallenge(ch, solved, u.IsAdmin()) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "errors": []string{"Challenge not found"}})
		return
	}
	if solved[ch.ID] {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "already_solved", "message": "You already solved this"}})
		return
	}
	if ch.MaxAttempts > 0 {
		q := a.DB.Model(&models.Submission{}).Where("challenge_id = ? AND type = ?", ch.ID, models.SubmissionIncorrect)
		if a.Store.IsTeams() && u.TeamID != nil {
			q = q.Where("team_id = ?", *u.TeamID)
		} else {
			q = q.Where("user_id = ?", u.ID)
		}
		var n int64
		q.Count(&n)
		if int(n) >= ch.MaxAttempts {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"You have 0 tries remaining"}})
			return
		}
	}
	ok := matchFlags(ch.Flags, provided)
	sub := models.Submission{
		ChallengeID: ch.ID, UserID: u.ID, TeamID: u.TeamID, IP: c.ClientIP(),
		Provided: provided, Date: time.Now(),
	}
	if ok {
		sub.Type = models.SubmissionCorrect
		_ = a.DB.Create(&sub).Error
		solve := models.Solve{
			ChallengeID: ch.ID, UserID: u.ID, TeamID: u.TeamID, IP: c.ClientIP(),
			Provided: provided, Date: time.Now(),
		}
		if err := a.DB.Create(&solve).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "already_solved", "message": "You already solved this"}})
			return
		}
		_ = scoring.RecalcChallenge(a.DB, &ch)
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "correct", "message": "Correct"}})
		return
	}
	sub.Type = models.SubmissionIncorrect
	_ = a.DB.Create(&sub).Error
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "incorrect", "message": "Incorrect"}})
}

func matchFlags(flags []models.Flag, provided string) bool {
	for _, f := range flags {
		content := f.Content
		got := provided
		if f.Data == "case_insensitive" {
			content = strings.ToLower(content)
			got = strings.ToLower(got)
		}
		switch f.Type {
		case models.FlagRegex:
			re, err := regexp.Compile(content)
			if err == nil && re.MatchString(provided) {
				return true
			}
		default:
			if got == content {
				return true
			}
		}
	}
	return false
}

func (a *App) UnlockHint(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "errors": []string{"Authentication required"}})
		return
	}
	if a.Store.IsTeams() && u.TeamID == nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"You must join a team first"}})
		return
	}
	var body struct {
		Target uint   `json:"target"`
		Type   string `json:"type"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Target == 0 {
		if id, err := strconv.Atoi(c.PostForm("target")); err == nil {
			body.Target = uint(id)
		}
	}
	if body.Type == "" {
		body.Type = models.UnlockHint
	}
	var hint models.Hint
	if err := a.DB.First(&hint, body.Target).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "errors": []string{"Hint not found"}})
		return
	}
	unlocked := scoring.UnlockedHintIDs(a.DB, u.ID, u.TeamID, a.Store.IsTeams())
	if unlocked[hint.ID] {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"content": hint.Content, "cost": hint.Cost}})
		return
	}
	un := models.Unlock{UserID: u.ID, TeamID: u.TeamID, Target: hint.ID, Type: models.UnlockHint, Date: time.Now()}
	if err := a.DB.Create(&un).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "errors": []string{"Could not unlock hint"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"content": hint.Content, "cost": hint.Cost}})
}

func (a *App) ChallengeSolvesJSON(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.canSeeScores(u) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"Forbidden"}})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	q := a.DB.Preload("User").Where("challenge_id = ?", id).Order("date ASC")
	if fr := a.freezeFor(u); fr != nil {
		q = q.Where("date <= ?", *fr)
	}
	var solves []models.Solve
	q.Find(&solves)
	var data []gin.H
	for i, s := range solves {
		name := s.User.Name
		if a.Store.IsTeams() && s.TeamID != nil {
			var t models.Team
			if a.DB.First(&t, *s.TeamID).Error == nil {
				name = t.Name
			}
		}
		if s.User.Hidden || s.User.Banned {
			continue
		}
		data = append(data, gin.H{"account_id": a.solveAccountID(s), "name": name, "date": s.Date, "place": i + 1})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (a *App) solveAccountID(s models.Solve) uint {
	if a.Store.IsTeams() && s.TeamID != nil {
		return *s.TeamID
	}
	return s.UserID
}

func (a *App) FileDownload(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var f models.File
	if err := a.DB.First(&f, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	u := middleware.CurrentUser(c)
	if f.ChallengeID != nil {
		if !a.canSeeChallenges(u) {
			c.Status(http.StatusForbidden)
			return
		}
		var ch models.Challenge
		if a.DB.First(&ch, *f.ChallengeID).Error != nil {
			c.Status(http.StatusNotFound)
			return
		}
		solved := map[uint]bool{}
		if u != nil {
			solved = scoring.SolvedIDs(a.DB, u.ID, u.TeamID, a.Store.IsTeams())
		}
		if !a.canAccessChallenge(ch, solved, u != nil && u.IsAdmin()) {
			c.Status(http.StatusForbidden)
			return
		}
	}
	path, err := webutil.UploadPath(a.Cfg.UploadDir, f.Location)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.FileAttachment(path, f.Name)
}
