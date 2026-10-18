package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/middleware"
	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/scoring"
	"github.com/gin-gonic/gin"
)

// APIChallenges godoc
// @Summary List challenges
// @Tags challenges
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/challenges [get]
func (a *App) APIChallenges(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.canSeeChallenges(u) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"Forbidden"}})
		return
	}
	if !a.Store.Started() && (u == nil || !u.IsAdmin()) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"CTF has not started yet"}})
		return
	}
	chs := a.visibleChallenges(u)
	solved := map[uint]bool{}
	if u != nil {
		solved = scoring.SolvedIDs(a.DB, u.ID, u.TeamID, a.Store.IsTeams())
	}
	var data []gin.H
	for _, ch := range chs {
		var n int64
		a.DB.Model(&models.Solve{}).Where("challenge_id = ?", ch.ID).Count(&n)
		data = append(data, gin.H{
			"id": ch.ID, "name": ch.Name, "value": ch.Value, "category": ch.Category,
			"type": ch.Type, "solves": n, "solved_by_me": solved[ch.ID], "tags": ch.Tags,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// APIChallenge godoc
// @Summary Get a challenge
// @Tags challenges
// @Produce json
// @Param id path int true "Challenge ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/challenges/{id} [get]
func (a *App) APIChallenge(c *gin.Context) {
	a.ChallengeJSON(c)
}

// APIAttempt godoc
// @Summary Submit a flag
// @Tags challenges
// @Accept json
// @Produce json
// @Param id path int true "Challenge ID"
// @Param body body object true "Flag submission"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/challenges/{id}/attempt [post]
func (a *App) APIAttempt(c *gin.Context) {
	a.AttemptFlag(c)
}

// APIChallengeSolves godoc
// @Summary List solves for a challenge
// @Tags challenges
// @Produce json
// @Param id path int true "Challenge ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/challenges/{id}/solves [get]
func (a *App) APIChallengeSolves(c *gin.Context) {
	a.ChallengeSolvesJSON(c)
}

// APIScoreboard godoc
// @Summary Get scoreboard
// @Tags scoreboard
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/scoreboard [get]
func (a *App) APIScoreboard(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.canSeeScores(u) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"Forbidden"}})
		return
	}
	standings := scoring.Scoreboard(a.DB, a.Store, a.freezeFor(u), u != nil && u.IsAdmin())
	c.JSON(http.StatusOK, gin.H{"success": true, "data": standings})
}

// APIScoreboardTop godoc
// @Summary Top N scoreboard graph data
// @Tags scoreboard
// @Produce json
// @Param count path int true "Count"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/scoreboard/top/{count} [get]
func (a *App) APIScoreboardTop(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.canSeeScores(u) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"Forbidden"}})
		return
	}
	n, _ := strconv.Atoi(c.Param("count"))
	if n <= 0 || n > 50 {
		n = 10
	}
	standings := scoring.Scoreboard(a.DB, a.Store, a.freezeFor(u), u != nil && u.IsAdmin())
	if len(standings) > n {
		standings = standings[:n]
	}
	ids := map[uint]bool{}
	for _, s := range standings {
		ids[s.AccountID] = true
	}
	type solveRow struct {
		ChallengeID uint
		UserID      uint
		TeamID      *uint
		Date        time.Time
		Value       int
		UserName    string
		TeamName    string
	}
	q := a.DB.Table("solves").
		Select("solves.challenge_id, solves.user_id, solves.team_id, solves.date, challenges.value, users.name as user_name, teams.name as team_name").
		Joins("JOIN challenges ON challenges.id = solves.challenge_id").
		Joins("JOIN users ON users.id = solves.user_id").
		Joins("LEFT JOIN teams ON teams.id = solves.team_id").
		Order("solves.date ASC")
	if fr := a.freezeFor(u); fr != nil {
		q = q.Where("solves.date <= ?", *fr)
	}
	var rows []solveRow
	q.Scan(&rows)
	graph := map[string][]gin.H{}
	scores := map[uint]int{}
	teams := a.Store.IsTeams()
	for _, r := range rows {
		id := r.UserID
		name := r.UserName
		if teams {
			if r.TeamID == nil {
				continue
			}
			id = *r.TeamID
			name = r.TeamName
		}
		if !ids[id] {
			continue
		}
		scores[id] += r.Value
		graph[name] = append(graph[name], gin.H{"date": r.Date, "score": scores[id]})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": graph})
}

// APIUsers godoc
// @Summary List users
// @Tags users
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/users [get]
func (a *App) APIUsers(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.canSeeAccounts(u) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"Forbidden"}})
		return
	}
	q := a.DB.Where("banned = ?", false)
	if u == nil || !u.IsAdmin() {
		q = q.Where("hidden = ?", false)
	}
	var users []models.User
	q.Order("id").Find(&users)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": users})
}

// APIUser godoc
// @Summary Get a user
// @Tags users
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/users/{id} [get]
func (a *App) APIUser(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.canSeeAccounts(u) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"Forbidden"}})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var user models.User
	if a.DB.Preload("Team").First(&user, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "errors": []string{"User not found"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": user})
}

// APITeams godoc
// @Summary List teams
// @Tags teams
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/teams [get]
func (a *App) APITeams(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.canSeeAccounts(u) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"Forbidden"}})
		return
	}
	var teams []models.Team
	q := a.DB.Preload("Members").Where("banned = ?", false)
	if u == nil || !u.IsAdmin() {
		q = q.Where("hidden = ?", false)
	}
	q.Order("id").Find(&teams)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": teams})
}

// APITeam godoc
// @Summary Get a team
// @Tags teams
// @Produce json
// @Param id path int true "Team ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/teams/{id} [get]
func (a *App) APITeam(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.canSeeAccounts(u) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"Forbidden"}})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var team models.Team
	if a.DB.Preload("Members").First(&team, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "errors": []string{"Team not found"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": team})
}

// APINotifications godoc
// @Summary List notifications
// @Tags notifications
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/notifications [get]
func (a *App) APINotifications(c *gin.Context) {
	var notes []models.Notification
	a.DB.Order("date DESC").Find(&notes)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": notes})
}

// APIUnlocks godoc
// @Summary Unlock a hint
// @Tags unlocks
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/unlocks [post]
func (a *App) APIUnlocks(c *gin.Context) {
	a.UnlockHint(c)
}

// APIConfig godoc
// @Summary Public config
// @Tags config
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/configs [get]
func (a *App) APIConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"ctf_name":                a.Store.CTFName(),
		"user_mode":               a.Store.UserMode(),
		"game_mode":               a.Store.GameMode(),
		"challenge_visibility":    a.Store.ChallengeVisibility(),
		"score_visibility":        a.Store.ScoreVisibility(),
		"account_visibility":      a.Store.AccountVisibility(),
		"registration_visibility": a.Store.RegistrationVisibility(),
	}})
}

// APIMe godoc
// @Summary Current user
// @Tags users
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/users/me [get]
func (a *App) APIMe(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "errors": []string{"Authentication required"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": u})
}
