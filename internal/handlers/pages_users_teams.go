package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/ad"
	"github.com/aapng-cmd/ctf_platform/internal/middleware"
	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/scoring"
	"github.com/aapng-cmd/ctf_platform/internal/webutil"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (a *App) Index(c *gin.Context) {
	var page models.Page
	if err := a.DB.Where("route = ?", "index").First(&page).Error; err != nil {
		a.HTML(c, http.StatusOK, "index.html", gin.H{"Title": a.Store.CTFName(), "Page": models.Page{
			Title: a.Store.CTFName(), Content: defaultIndex(a.Store.CTFName(), a.Store.Get("ctf_description")),
		}, "Flashes": a.flashes(c)})
		return
	}
	a.HTML(c, http.StatusOK, "index.html", gin.H{"Title": page.Title, "Page": page, "Flashes": a.flashes(c)})
}

func (a *App) CustomPage(c *gin.Context) {
	route := strings.TrimPrefix(c.Param("route"), "/")
	if route == "" {
		a.Index(c)
		return
	}
	var page models.Page
	if err := a.DB.Where("route = ? AND draft = ?", route, false).First(&page).Error; err != nil {
		a.HTML(c, http.StatusNotFound, "error.html", gin.H{"Title": "404", "Message": "Page not found"})
		return
	}
	if page.Hidden && (middleware.CurrentUser(c) == nil || !middleware.CurrentUser(c).IsAdmin()) {
		a.HTML(c, http.StatusNotFound, "error.html", gin.H{"Title": "404", "Message": "Page not found"})
		return
	}
	if page.AuthRequired && middleware.CurrentUser(c) == nil {
		c.Redirect(http.StatusFound, "/login?next="+webutil.SafeNext("/"+route, "/"))
		return
	}
	a.HTML(c, http.StatusOK, "index.html", gin.H{"Title": page.Title, "Page": page, "Flashes": a.flashes(c)})
}

func (a *App) Notifications(c *gin.Context) {
	var notes []models.Notification
	a.DB.Order("date DESC").Find(&notes)
	a.HTML(c, http.StatusOK, "notifications.html", gin.H{"Title": "Notifications", "Notifications": notes, "Flashes": a.flashes(c)})
}

func (a *App) UsersList(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.canSeeAccounts(u) {
		a.HTML(c, http.StatusForbidden, "error.html", gin.H{"Title": "Forbidden", "Message": "User listing is not available."})
		return
	}
	q := a.DB.Model(&models.User{}).Where("banned = ?", false)
	if u == nil || !u.IsAdmin() {
		q = q.Where("hidden = ?", false)
	}
	var users []models.User
	q.Order("name").Find(&users)
	a.HTML(c, http.StatusOK, "users.html", gin.H{
		"Title": "Users", "Users": users, "Flashes": a.flashes(c),
	})
}

func (a *App) UserDetail(c *gin.Context) {
	viewer := middleware.CurrentUser(c)
	if !a.canSeeAccounts(viewer) {
		a.HTML(c, http.StatusForbidden, "error.html", gin.H{"Title": "Forbidden", "Message": "Accounts are not visible."})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var user models.User
	if err := a.DB.Preload("Team").First(&user, id).Error; err != nil {
		a.HTML(c, http.StatusNotFound, "error.html", gin.H{"Title": "404", "Message": "User not found"})
		return
	}
	if (user.Hidden || user.Banned) && (viewer == nil || !viewer.IsAdmin()) && (viewer == nil || viewer.ID != user.ID) {
		a.HTML(c, http.StatusNotFound, "error.html", gin.H{"Title": "404", "Message": "User not found"})
		return
	}
	a.renderProfile(c, viewer, user)
}

func (a *App) TeamsList(c *gin.Context) {
	if !a.Store.IsTeams() {
		c.Redirect(http.StatusFound, "/users")
		return
	}
	u := middleware.CurrentUser(c)
	if !a.canSeeAccounts(u) {
		a.HTML(c, http.StatusForbidden, "error.html", gin.H{"Title": "Forbidden", "Message": "Team listing is not available."})
		return
	}
	q := a.DB.Preload("Members").Preload("Roster").Where("banned = ?", false)
	if u == nil || !u.IsAdmin() {
		q = q.Where("hidden = ?", false)
	}
	var teams []models.Team
	q.Order("name").Find(&teams)
	a.HTML(c, http.StatusOK, "teams.html", gin.H{"Title": "Teams", "Teams": teams, "Flashes": a.flashes(c)})
}

func (a *App) TeamDetail(c *gin.Context) {
	if !a.Store.IsTeams() {
		c.Redirect(http.StatusFound, "/users")
		return
	}
	viewer := middleware.CurrentUser(c)
	if !a.canSeeAccounts(viewer) {
		a.HTML(c, http.StatusForbidden, "error.html", gin.H{"Title": "Forbidden", "Message": "Accounts are not visible."})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var team models.Team
	if err := a.DB.Preload("Members").Preload("Roster").First(&team, id).Error; err != nil {
		a.HTML(c, http.StatusNotFound, "error.html", gin.H{"Title": "404", "Message": "Team not found"})
		return
	}
	if (team.Hidden || team.Banned) && (viewer == nil || !viewer.IsAdmin()) {
		a.HTML(c, http.StatusNotFound, "error.html", gin.H{"Title": "404", "Message": "Team not found"})
		return
	}
	a.HTML(c, http.StatusOK, "team.html", gin.H{
		"Title": team.Name, "Team": team, "Flashes": a.flashes(c),
	})
}

func (a *App) TeamNewGET(c *gin.Context) {
	if !a.Store.IsTeams() {
		c.Redirect(http.StatusFound, "/")
		return
	}
	if !a.Store.IsAD() {
		a.flash(c, "warning", "Teams are only open during Attack-Defense")
		c.Redirect(http.StatusFound, "/teams")
		return
	}
	a.HTML(c, http.StatusOK, "team_form.html", gin.H{"Title": "Create Team", "Mode": "create", "Flashes": a.flashes(c)})
}

func (a *App) TeamNewPOST(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.Store.IsAD() {
		a.flash(c, "warning", "Teams are only open during Attack-Defense")
		c.Redirect(http.StatusFound, "/teams")
		return
	}
	if u.Banned {
		a.flash(c, "danger", "You have been banned")
		c.Redirect(http.StatusFound, "/teams")
		return
	}
	if u.TeamID != nil {
		a.flash(c, "warning", "You are already on a team")
		c.Redirect(http.StatusFound, "/teams/"+strconv.Itoa(int(*u.TeamID)))
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	password := c.PostForm("password")
	if !a.AuthLimiter.Allow("team-new:" + c.ClientIP()) {
		a.flash(c, "danger", "Too many attempts. Wait a minute and try again.")
		c.Redirect(http.StatusFound, "/teams/new")
		return
	}
	if !webutil.ValidName(name) || len(password) < 6 {
		a.flash(c, "danger", "Team name is required. Password must be at least 6 characters.")
		c.Redirect(http.StatusFound, "/teams/new")
		return
	}
	hash, _ := webutil.HashPassword(password)
	team := models.Team{
		Name: name, Password: hash, Website: strings.TrimSpace(c.PostForm("website")),
		Affiliation: strings.TrimSpace(c.PostForm("affiliation")), Country: strings.TrimSpace(c.PostForm("country")),
		Created: time.Now(), EventID: uint(a.Store.Int("ad_event_id", 0)),
	}
	if err := a.DB.Create(&team).Error; err != nil {
		a.flash(c, "danger", "That team name is already taken")
		c.Redirect(http.StatusFound, "/teams/new")
		return
	}
	cap := team.ID
	_ = a.DB.Model(&team).Update("captain_id", u.ID).Error
	_ = cap
	team.CaptainID = &u.ID
	a.DB.Save(&team)
	a.DB.Model(u).Update("team_id", team.ID)
	a.rememberRoster(team.ID, u.ID, u.Name)
	_ = ad.EnsureInstances(a.DB)
	c.Redirect(http.StatusFound, "/teams/"+strconv.Itoa(int(team.ID)))
}

func (a *App) TeamJoinGET(c *gin.Context) {
	if !a.Store.IsTeams() {
		c.Redirect(http.StatusFound, "/")
		return
	}
	if !a.Store.IsAD() {
		a.flash(c, "warning", "Teams are only open during Attack-Defense")
		c.Redirect(http.StatusFound, "/teams")
		return
	}
	a.HTML(c, http.StatusOK, "team_form.html", gin.H{"Title": "Join Team", "Mode": "join", "Flashes": a.flashes(c)})
}

func (a *App) TeamJoinPOST(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.Store.IsAD() {
		a.flash(c, "warning", "Teams are only open during Attack-Defense")
		c.Redirect(http.StatusFound, "/teams")
		return
	}
	if u.Banned {
		a.flash(c, "danger", "You have been banned")
		c.Redirect(http.StatusFound, "/teams")
		return
	}
	if u.TeamID != nil {
		a.flash(c, "warning", "You are already on a team")
		c.Redirect(http.StatusFound, "/teams/"+strconv.Itoa(int(*u.TeamID)))
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	password := c.PostForm("password")
	if !a.AuthLimiter.Allow("team-join:" + c.ClientIP() + ":" + strings.ToLower(name)) {
		a.flash(c, "danger", "Too many attempts. Wait a minute and try again.")
		c.Redirect(http.StatusFound, "/teams/join")
		return
	}
	var team models.Team
	err := a.DB.Preload("Members").Where("name = ?", name).First(&team).Error
	if !webutil.CheckPassword(team.Password, password) || err != nil {
		a.flash(c, "danger", "That team name or password is incorrect")
		c.Redirect(http.StatusFound, "/teams/join")
		return
	}
	if team.Banned || team.Closed || (team.EventID != 0 && team.EventID != uint(a.Store.Int("ad_event_id", 0))) {
		a.flash(c, "danger", "That team is closed")
		c.Redirect(http.StatusFound, "/teams/join")
		return
	}
	if max := a.Store.MaxTeamSize(); max > 0 && len(team.Members) >= max {
		a.flash(c, "danger", "That team is already full")
		c.Redirect(http.StatusFound, "/teams/join")
		return
	}
	if team.EventID == 0 {
		a.DB.Model(&team).Update("event_id", uint(a.Store.Int("ad_event_id", 0)))
	}
	a.DB.Model(u).Update("team_id", team.ID)
	a.rememberRoster(team.ID, u.ID, u.Name)
	c.Redirect(http.StatusFound, "/teams/"+strconv.Itoa(int(team.ID)))
}

func (a *App) rememberRoster(teamID, userID uint, name string) {
	var row models.TeamRoster
	err := a.DB.Where("team_id = ? AND user_id = ?", teamID, userID).First(&row).Error
	if err != nil {
		a.DB.Create(&models.TeamRoster{TeamID: teamID, UserID: userID, Name: name})
		return
	}
	if row.Name != name {
		a.DB.Model(&row).Update("name", name)
	}
}

// closeOpenTeams finishes every team that is still open. eventID, when set,
// is written onto teams that were not tied to an event yet.
func (a *App) closeOpenTeams(eventID uint) {
	var users []models.User
	a.DB.Where("team_id IS NOT NULL").Find(&users)
	for _, u := range users {
		if u.TeamID != nil {
			a.rememberRoster(*u.TeamID, u.ID, u.Name)
		}
	}
	if eventID != 0 {
		a.DB.Model(&models.Team{}).Where("closed = ? AND event_id = ?", false, 0).Update("event_id", eventID)
	}
	a.DB.Model(&models.Team{}).Where("closed = ?", false).Update("closed", true)
	a.DB.Model(&models.User{}).Where("team_id IS NOT NULL").Update("team_id", gorm.Expr("NULL"))
}

func (a *App) ScoreboardPage(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.canSeeScores(u) {
		a.HTML(c, http.StatusForbidden, "error.html", gin.H{"Title": "Scoreboard", "Message": "Scoreboard is not available."})
		return
	}
	standings := scoring.UserScoreboard(a.DB, nil, u != nil && u.IsAdmin())
	var adStandings []scoring.ADStanding
	if a.Store.IsAD() {
		adStandings = a.adBoard(u != nil && u.IsAdmin()).Teams
	}
	a.HTML(c, http.StatusOK, "scoreboard.html", gin.H{
		"Title": "Scoreboard", "Standings": standings, "ADStandings": adStandings, "Flashes": a.flashes(c),
	})
}
