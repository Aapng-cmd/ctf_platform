package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/scoring"
	"github.com/aapng-cmd/ctf_platform/internal/webutil"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (a *App) AdminHome(c *gin.Context) {
	c.Redirect(http.StatusFound, "/admin/statistics")
}

func (a *App) AdminStatistics(c *gin.Context) {
	var users, teams, challenges, solves, submissions int64
	a.DB.Model(&models.User{}).Count(&users)
	a.DB.Model(&models.Team{}).Count(&teams)
	a.DB.Model(&models.Challenge{}).Count(&challenges)
	a.DB.Model(&models.Solve{}).Count(&solves)
	a.DB.Model(&models.Submission{}).Count(&submissions)
	var byCat []struct {
		Category string
		Count    int64
	}
	a.DB.Model(&models.Challenge{}).Select("category, count(*) as count").Group("category").Scan(&byCat)
	standings := scoring.Scoreboard(a.DB, a.Store, nil, true)
	top := standings
	if len(top) > 10 {
		top = top[:10]
	}
	a.HTML(c, http.StatusOK, "admin_statistics.html", gin.H{
		"Title": "Statistics", "Admin": true, "Users": users, "Teams": teams,
		"Challenges": challenges, "Solves": solves, "Submissions": submissions,
		"ByCategory": byCat, "Top": top, "Flashes": a.flashes(c),
	})
}

func (a *App) AdminChallenges(c *gin.Context) {
	var chs []models.Challenge
	a.DB.Preload("Tags").Order("id DESC").Find(&chs)
	counts := map[uint]int64{}
	for _, ch := range chs {
		var n int64
		a.DB.Model(&models.Solve{}).Where("challenge_id = ?", ch.ID).Count(&n)
		counts[ch.ID] = n
	}
	a.HTML(c, http.StatusOK, "admin_challenges.html", gin.H{
		"Title": "Challenges", "Admin": true, "Challenges": chs, "SolveCounts": counts, "Flashes": a.flashes(c),
	})
}

func (a *App) AdminChallengeNewGET(c *gin.Context) {
	a.HTML(c, http.StatusOK, "admin_challenge_edit.html", gin.H{
		"Title": "New Challenge", "Admin": true, "Challenge": models.Challenge{Type: models.ChallengeStandard, State: models.StateVisible, Value: 100, Function: "linear"},
		"Flashes": a.flashes(c),
	})
}

func (a *App) AdminChallengeEditGET(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ch models.Challenge
	if err := a.DB.Preload("Flags").Preload("Hints").Preload("Files").Preload("Tags").First(&ch, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/challenges")
		return
	}
	a.HTML(c, http.StatusOK, "admin_challenge_edit.html", gin.H{
		"Title": "Edit Challenge", "Admin": true, "Challenge": ch, "Flashes": a.flashes(c),
	})
}

func (a *App) parseChallengeForm(c *gin.Context, ch *models.Challenge) {
	ch.Name = strings.TrimSpace(c.PostForm("name"))
	ch.Category = strings.TrimSpace(c.PostForm("category"))
	ch.Description = c.PostForm("description")
	ch.Attribution = strings.TrimSpace(c.PostForm("attribution"))
	ch.ConnectionInfo = strings.TrimSpace(c.PostForm("connection_info"))
	ch.Type = c.PostForm("type")
	if ch.Type != models.ChallengeDynamic {
		ch.Type = models.ChallengeStandard
	}
	ch.State = c.PostForm("state")
	if ch.State != models.StateHidden {
		ch.State = models.StateVisible
	}
	ch.Value, _ = strconv.Atoi(c.PostForm("value"))
	ch.MaxAttempts, _ = strconv.Atoi(c.PostForm("max_attempts"))
	ch.Initial, _ = strconv.Atoi(c.PostForm("initial"))
	ch.Minimum, _ = strconv.Atoi(c.PostForm("minimum"))
	ch.Decay, _ = strconv.Atoi(c.PostForm("decay"))
	ch.Function = c.PostForm("function")
	if ch.Function == "" {
		ch.Function = "linear"
	}
	if req := strings.TrimSpace(c.PostForm("requirements")); req != "" {
		parts := strings.Split(req, ",")
		var ids []uint
		for _, p := range parts {
			if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && n > 0 {
				ids = append(ids, uint(n))
			}
		}
		b, _ := json.Marshal(ids)
		ch.Requirements = string(b)
	} else {
		ch.Requirements = ""
	}
	if ch.Type == models.ChallengeDynamic {
		if ch.Initial == 0 {
			ch.Initial = ch.Value
		}
		if ch.Value == 0 {
			ch.Value = ch.Initial
		}
	}
}

func (a *App) AdminChallengeNewPOST(c *gin.Context) {
	ch := models.Challenge{Created: time.Now()}
	a.parseChallengeForm(c, &ch)
	if ch.Name == "" || ch.Category == "" {
		a.flash(c, "danger", "Name and category are required")
		c.Redirect(http.StatusFound, "/admin/challenges/new")
		return
	}
	if err := a.DB.Create(&ch).Error; err != nil {
		a.flash(c, "danger", "Could not create challenge")
		c.Redirect(http.StatusFound, "/admin/challenges/new")
		return
	}
	a.saveChallengeExtras(c, &ch)
	a.flash(c, "success", "Challenge created")
	c.Redirect(http.StatusFound, "/admin/challenges/"+strconv.Itoa(int(ch.ID)))
}

func (a *App) AdminChallengeEditPOST(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ch models.Challenge
	if err := a.DB.First(&ch, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/challenges")
		return
	}
	a.parseChallengeForm(c, &ch)
	if err := a.DB.Save(&ch).Error; err != nil {
		a.flash(c, "danger", "Could not save challenge")
	} else {
		a.saveChallengeExtras(c, &ch)
		_ = scoring.RecalcChallenge(a.DB, &ch)
		a.flash(c, "success", "Challenge updated")
	}
	c.Redirect(http.StatusFound, "/admin/challenges/"+strconv.Itoa(id))
}

func (a *App) saveChallengeExtras(c *gin.Context, ch *models.Challenge) {
	if flags := c.PostFormArray("flag_content"); len(flags) > 0 {
		a.DB.Where("challenge_id = ?", ch.ID).Delete(&models.Flag{})
		types := c.PostFormArray("flag_type")
		datas := c.PostFormArray("flag_data")
		for i, content := range flags {
			content = strings.TrimSpace(content)
			if content == "" {
				continue
			}
			ft, fd := models.FlagStatic, ""
			if i < len(types) {
				ft = types[i]
			}
			if i < len(datas) {
				fd = datas[i]
			}
			a.DB.Create(&models.Flag{ChallengeID: ch.ID, Type: ft, Content: content, Data: fd})
		}
	}
	if titles := c.PostFormArray("hint_title"); len(titles) > 0 {
		a.DB.Where("challenge_id = ?", ch.ID).Delete(&models.Hint{})
		contents := c.PostFormArray("hint_content")
		costs := c.PostFormArray("hint_cost")
		for i, title := range titles {
			content := ""
			if i < len(contents) {
				content = contents[i]
			}
			if strings.TrimSpace(content) == "" && strings.TrimSpace(title) == "" {
				continue
			}
			cost := 0
			if i < len(costs) {
				cost, _ = strconv.Atoi(costs[i])
			}
			a.DB.Create(&models.Hint{ChallengeID: ch.ID, Title: title, Content: content, Cost: cost})
		}
	}
	if tags := strings.TrimSpace(c.PostForm("tags")); true {
		a.DB.Where("challenge_id = ?", ch.ID).Delete(&models.Tag{})
		for _, t := range strings.Split(tags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				a.DB.Create(&models.Tag{ChallengeID: ch.ID, Value: t})
			}
		}
	}
	form, err := c.MultipartForm()
	if err == nil && form.File["files"] != nil {
		_ = os.MkdirAll(a.Cfg.UploadDir, 0o755)
		for _, fh := range form.File["files"] {
			src, err := fh.Open()
			if err != nil {
				continue
			}
			name := strconv.FormatInt(time.Now().UnixNano(), 10) + "_" + filepath.Base(fh.Filename)
			dstPath := filepath.Join(a.Cfg.UploadDir, name)
			dst, err := os.Create(dstPath)
			if err != nil {
				_ = src.Close()
				continue
			}
			_, _ = io.Copy(dst, src)
			_ = dst.Close()
			_ = src.Close()
			a.DB.Create(&models.File{Type: "challenge", Location: name, Name: fh.Filename, ChallengeID: &ch.ID})
		}
	}
}

func (a *App) AdminChallengeDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.DB.Where("challenge_id = ?", id).Delete(&models.Flag{})
	a.DB.Where("challenge_id = ?", id).Delete(&models.Hint{})
	a.DB.Where("challenge_id = ?", id).Delete(&models.Tag{})
	a.DB.Where("challenge_id = ?", id).Delete(&models.File{})
	a.DB.Where("challenge_id = ?", id).Delete(&models.Solve{})
	a.DB.Where("challenge_id = ?", id).Delete(&models.Submission{})
	a.DB.Delete(&models.Challenge{}, id)
	a.flash(c, "success", "Challenge deleted")
	c.Redirect(http.StatusFound, "/admin/challenges")
}

func (a *App) AdminFileDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var f models.File
	if a.DB.First(&f, id).Error == nil {
		_ = os.Remove(filepath.Join(a.Cfg.UploadDir, f.Location))
		a.DB.Delete(&f)
	}
	c.Redirect(http.StatusFound, c.GetHeader("Referer"))
}

func (a *App) AdminUsers(c *gin.Context) {
	var users []models.User
	a.DB.Preload("Team").Order("id").Find(&users)
	a.HTML(c, http.StatusOK, "admin_users.html", gin.H{"Title": "Users", "Admin": true, "Users": users, "Flashes": a.flashes(c)})
}

func (a *App) AdminUserEditGET(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var user models.User
	if err := a.DB.Preload("Team").First(&user, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/users")
		return
	}
	var solves []models.Solve
	a.DB.Preload("Challenge").Where("user_id = ?", user.ID).Order("date DESC").Find(&solves)
	a.HTML(c, http.StatusOK, "admin_user_edit.html", gin.H{"Title": "Edit User", "Admin": true, "Profile": user, "Solves": solves, "Flashes": a.flashes(c)})
}

func (a *App) AdminUserEditPOST(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var user models.User
	if a.DB.First(&user, id).Error != nil {
		c.Redirect(http.StatusFound, "/admin/users")
		return
	}
	updates := map[string]any{
		"name":        strings.TrimSpace(c.PostForm("name")),
		"email":       strings.TrimSpace(c.PostForm("email")),
		"type":        c.PostForm("type"),
		"website":     c.PostForm("website"),
		"affiliation": c.PostForm("affiliation"),
		"country":     c.PostForm("country"),
		"hidden":      c.PostForm("hidden") == "on",
		"banned":      c.PostForm("banned") == "on",
		"verified":    c.PostForm("verified") == "on",
	}
	if p := c.PostForm("password"); p != "" {
		if hash, err := webutil.HashPassword(p); err == nil {
			updates["password"] = hash
		}
	}
	a.DB.Model(&user).Updates(updates)
	a.flash(c, "success", "User updated")
	c.Redirect(http.StatusFound, "/admin/users/"+strconv.Itoa(id))
}

func (a *App) AdminUserDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.DB.Delete(&models.User{}, id)
	a.flash(c, "success", "User deleted")
	c.Redirect(http.StatusFound, "/admin/users")
}

func (a *App) AdminTeams(c *gin.Context) {
	var teams []models.Team
	a.DB.Preload("Members").Order("id").Find(&teams)
	a.HTML(c, http.StatusOK, "admin_teams.html", gin.H{"Title": "Teams", "Admin": true, "Teams": teams, "Flashes": a.flashes(c)})
}

func (a *App) AdminTeamEditGET(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var team models.Team
	if a.DB.Preload("Members").First(&team, id).Error != nil {
		c.Redirect(http.StatusFound, "/admin/teams")
		return
	}
	a.HTML(c, http.StatusOK, "admin_team_edit.html", gin.H{"Title": "Edit Team", "Admin": true, "Team": team, "Flashes": a.flashes(c)})
}

func (a *App) AdminTeamEditPOST(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var team models.Team
	if a.DB.First(&team, id).Error != nil {
		c.Redirect(http.StatusFound, "/admin/teams")
		return
	}
	updates := map[string]any{
		"name":        strings.TrimSpace(c.PostForm("name")),
		"email":       c.PostForm("email"),
		"website":     c.PostForm("website"),
		"affiliation": c.PostForm("affiliation"),
		"country":     c.PostForm("country"),
		"hidden":      c.PostForm("hidden") == "on",
		"banned":      c.PostForm("banned") == "on",
	}
	if p := c.PostForm("password"); p != "" {
		if hash, err := webutil.HashPassword(p); err == nil {
			updates["password"] = hash
		}
	}
	a.DB.Model(&team).Updates(updates)
	a.flash(c, "success", "Team updated")
	c.Redirect(http.StatusFound, "/admin/teams/"+strconv.Itoa(id))
}

func (a *App) AdminTeamDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.DB.Model(&models.User{}).Where("team_id = ?", id).Update("team_id", nil)
	a.DB.Delete(&models.Team{}, id)
	a.flash(c, "success", "Team deleted")
	c.Redirect(http.StatusFound, "/admin/teams")
}

func (a *App) AdminSubmissions(c *gin.Context) {
	kind := c.DefaultQuery("type", "incorrect")
	q := a.DB.Preload("User").Preload("Challenge").Order("date DESC").Limit(500)
	if kind != "all" {
		q = q.Where("type = ?", kind)
	}
	var subs []models.Submission
	q.Find(&subs)
	a.HTML(c, http.StatusOK, "admin_submissions.html", gin.H{
		"Title": "Submissions", "Admin": true, "Submissions": subs, "Kind": kind, "Flashes": a.flashes(c),
	})
}

func (a *App) AdminSubmissionDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.DB.Delete(&models.Submission{}, id)
	c.Redirect(http.StatusFound, "/admin/submissions")
}

func (a *App) AdminPages(c *gin.Context) {
	var pages []models.Page
	a.DB.Order("id").Find(&pages)
	a.HTML(c, http.StatusOK, "admin_pages.html", gin.H{"Title": "Pages", "Admin": true, "Pages": pages, "Flashes": a.flashes(c)})
}

func (a *App) AdminPageNewGET(c *gin.Context) {
	a.HTML(c, http.StatusOK, "admin_page_edit.html", gin.H{"Title": "New Page", "Admin": true, "Page": models.Page{}, "Flashes": a.flashes(c)})
}

func (a *App) AdminPageEditGET(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var page models.Page
	if a.DB.First(&page, id).Error != nil {
		c.Redirect(http.StatusFound, "/admin/pages")
		return
	}
	a.HTML(c, http.StatusOK, "admin_page_edit.html", gin.H{"Title": "Edit Page", "Admin": true, "Page": page, "Flashes": a.flashes(c)})
}

func (a *App) savePage(c *gin.Context, page *models.Page) {
	page.Title = strings.TrimSpace(c.PostForm("title"))
	page.Route = strings.Trim(strings.TrimSpace(c.PostForm("route")), "/")
	page.Content = c.PostForm("content")
	page.Draft = c.PostForm("draft") == "on"
	page.Hidden = c.PostForm("hidden") == "on"
	page.AuthRequired = c.PostForm("auth_required") == "on"
}

func (a *App) AdminPageNewPOST(c *gin.Context) {
	var page models.Page
	a.savePage(c, &page)
	if page.Title == "" || page.Route == "" {
		a.flash(c, "danger", "Title and route are required")
		c.Redirect(http.StatusFound, "/admin/pages/new")
		return
	}
	if err := a.DB.Create(&page).Error; err != nil {
		a.flash(c, "danger", "Could not create page")
		c.Redirect(http.StatusFound, "/admin/pages/new")
		return
	}
	a.flash(c, "success", "Page created")
	c.Redirect(http.StatusFound, "/admin/pages")
}

func (a *App) AdminPageEditPOST(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var page models.Page
	if a.DB.First(&page, id).Error != nil {
		c.Redirect(http.StatusFound, "/admin/pages")
		return
	}
	a.savePage(c, &page)
	a.DB.Save(&page)
	a.flash(c, "success", "Page updated")
	c.Redirect(http.StatusFound, "/admin/pages/"+strconv.Itoa(id))
}

func (a *App) AdminPageDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.DB.Delete(&models.Page{}, id)
	a.flash(c, "success", "Page deleted")
	c.Redirect(http.StatusFound, "/admin/pages")
}

func (a *App) AdminNotifications(c *gin.Context) {
	var notes []models.Notification
	a.DB.Order("date DESC").Find(&notes)
	a.HTML(c, http.StatusOK, "admin_notifications.html", gin.H{"Title": "Notifications", "Admin": true, "Notifications": notes, "Flashes": a.flashes(c)})
}

func (a *App) AdminNotificationCreate(c *gin.Context) {
	n := models.Notification{Title: strings.TrimSpace(c.PostForm("title")), Content: c.PostForm("content"), Date: time.Now()}
	if n.Title == "" {
		a.flash(c, "danger", "Title is required")
	} else {
		a.DB.Create(&n)
		a.flash(c, "success", "Notification posted")
	}
	c.Redirect(http.StatusFound, "/admin/notifications")
}

func (a *App) AdminNotificationDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.DB.Delete(&models.Notification{}, id)
	c.Redirect(http.StatusFound, "/admin/notifications")
}

func (a *App) AdminAwards(c *gin.Context) {
	var awards []models.Award
	a.DB.Preload("User").Order("date DESC").Find(&awards)
	var users []models.User
	a.DB.Order("name").Find(&users)
	a.HTML(c, http.StatusOK, "admin_awards.html", gin.H{"Title": "Awards", "Admin": true, "Awards": awards, "Users": users, "Flashes": a.flashes(c)})
}

func (a *App) AdminAwardCreate(c *gin.Context) {
	uid, _ := strconv.Atoi(c.PostForm("user_id"))
	val, _ := strconv.Atoi(c.PostForm("value"))
	var user models.User
	if a.DB.First(&user, uid).Error != nil {
		a.flash(c, "danger", "User not found")
		c.Redirect(http.StatusFound, "/admin/awards")
		return
	}
	aw := models.Award{
		UserID: uint(uid), TeamID: user.TeamID, Name: strings.TrimSpace(c.PostForm("name")),
		Description: c.PostForm("description"), Value: val, Category: c.PostForm("category"),
		Icon: c.PostForm("icon"), Date: time.Now(),
	}
	a.DB.Create(&aw)
	a.flash(c, "success", "Award granted")
	c.Redirect(http.StatusFound, "/admin/awards")
}

func (a *App) AdminAwardDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.DB.Delete(&models.Award{}, id)
	c.Redirect(http.StatusFound, "/admin/awards")
}

func (a *App) AdminConfigGET(c *gin.Context) {
	loc := a.Store.Location()
	a.HTML(c, http.StatusOK, "admin_config.html", gin.H{
		"Title": "Config", "Admin": true, "Config": map[string]string{
			"ctf_name":                a.Store.Get("ctf_name"),
			"ctf_description":         a.Store.Get("ctf_description"),
			"user_mode":               a.Store.UserMode(),
			"game_mode":               a.Store.GameMode(),
			"timezone":                a.Store.Timezone(),
			"start":                   clockInput(a.Store.Start(), loc),
			"end":                     clockInput(a.Store.End(), loc),
			"freeze":                  clockInput(a.Store.Freeze(), loc),
			"challenge_visibility":    a.Store.ChallengeVisibility(),
			"score_visibility":        a.Store.ScoreVisibility(),
			"account_visibility":      a.Store.AccountVisibility(),
			"registration_visibility": a.Store.RegistrationVisibility(),
			"max_team_size":           a.Store.GetDefault("max_team_size", "0"),
		},
		"ZoneNow": time.Now().In(loc).Format("2006-01-02 15:04 MST"),
		"Flashes": a.flashes(c),
	})
}

func (a *App) AdminConfigPOST(c *gin.Context) {
	loc, zone, err := loadZone(c.PostForm("timezone"))
	if err != nil {
		a.flash(c, "danger", "Unknown timezone")
		c.Redirect(http.StatusFound, "/admin/config")
		return
	}
	keys := []string{
		"ctf_name", "ctf_description", "user_mode", "game_mode",
		"challenge_visibility", "score_visibility", "account_visibility",
		"registration_visibility", "max_team_size",
	}
	pairs := map[string]string{"timezone": zone}
	for _, k := range keys {
		pairs[k] = c.PostForm(k)
	}
	for _, k := range []string{"start", "end", "freeze"} {
		pairs[k] = clockValue(c.PostForm(k), loc)
	}
	if pairs["game_mode"] == models.GameAttackDefense || pairs["game_mode"] == models.GameMixed {
		pairs["user_mode"] = models.ModeTeams
	}
	if err := a.Store.SetMany(pairs); err != nil {
		a.flash(c, "danger", "Could not save config")
	} else {
		a.flash(c, "success", "Configuration saved")
	}
	c.Redirect(http.StatusFound, "/admin/config")
}

func (a *App) AdminScoreboard(c *gin.Context) {
	standings := scoring.Scoreboard(a.DB, a.Store, nil, true)
	a.HTML(c, http.StatusOK, "admin_scoreboard.html", gin.H{"Title": "Scoreboard", "Admin": true, "Standings": standings, "Flashes": a.flashes(c)})
}

func (a *App) AdminManualSolve(c *gin.Context) {
	uid, _ := strconv.Atoi(c.PostForm("user_id"))
	cid, _ := strconv.Atoi(c.PostForm("challenge_id"))
	var user models.User
	var ch models.Challenge
	if a.DB.First(&user, uid).Error != nil || a.DB.First(&ch, cid).Error != nil {
		a.flash(c, "danger", "User or challenge not found")
		c.Redirect(http.StatusFound, c.GetHeader("Referer"))
		return
	}
	solve := models.Solve{ChallengeID: ch.ID, UserID: user.ID, TeamID: user.TeamID, Provided: "admin", Date: time.Now(), IP: c.ClientIP()}
	if err := a.DB.Create(&solve).Error; err != nil {
		a.flash(c, "warning", "Already solved")
	} else {
		_ = scoring.RecalcChallenge(a.DB, &ch)
		a.flash(c, "success", "Solve awarded")
	}
	c.Redirect(http.StatusFound, c.GetHeader("Referer"))
}

func (a *App) AdminReset(c *gin.Context) {
	if c.PostForm("confirm") != "RESET" {
		a.flash(c, "danger", "Type RESET to confirm")
		c.Redirect(http.StatusFound, "/admin/config")
		return
	}
	_ = a.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.Solve{}).Error
	_ = a.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.Submission{}).Error
	_ = a.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.Unlock{}).Error
	_ = a.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.Award{}).Error
	_ = a.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.ADSteal{}).Error
	_ = a.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.ADCheck{}).Error
	if a.Flags != nil {
		a.Flags.DeleteAll()
	}
	_ = a.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.ADRound{}).Error
	a.flash(c, "success", "Solves, submissions, unlocks, awards and Attack-Defense rounds were wiped")
	c.Redirect(http.StatusFound, "/admin/config")
}
