package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/ad"
	"github.com/aapng-cmd/ctf_platform/internal/middleware"
	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/scoring"
	"github.com/aapng-cmd/ctf_platform/internal/tasks"
	"github.com/aapng-cmd/ctf_platform/internal/webutil"
	"github.com/gin-gonic/gin"
)

func (a *App) AttackPage(c *gin.Context) {
	if !a.Store.IsAD() {
		a.HTML(c, http.StatusNotFound, "error.html", gin.H{"Title": "Attack-Defense", "Message": "Attack-Defense is not enabled."})
		return
	}
	u := middleware.CurrentUser(c)
	if u == nil {
		c.Redirect(http.StatusFound, "/login?next=/attack")
		return
	}
	if u.TeamID == nil && !u.IsAdmin() {
		a.HTML(c, http.StatusForbidden, "error.html", gin.H{"Title": "Attack-Defense", "Message": "Join a team to play Attack-Defense."})
		return
	}
	_ = ad.EnsureInstances(a.DB)
	round := ad.CurrentRound(a.DB)
	var insts []models.ADInstance
	a.DB.Joins("Team").Joins("Service").
		Where(`"Service"."active" = ? AND "Team"."banned" = ? AND "Team"."hidden" = ?`, true, false, false).
		Order(`"Team"."name", "Service"."id"`).Find(&insts)
	var mine []models.ADInstance
	if u.TeamID != nil {
		a.DB.Preload("Service").Where("team_id = ?", *u.TeamID).Find(&mine)
	}
	var steals []models.ADSteal
	q := a.DB.Preload("Attacker").Preload("Victim").Order("date DESC").Limit(40)
	if u.TeamID != nil {
		q = q.Where("attacker_team_id = ? OR victim_team_id = ?", *u.TeamID, *u.TeamID)
	}
	q.Find(&steals)
	serviceNames := map[uint]string{}
	var services []models.ADService
	a.DB.Find(&services)
	for _, s := range services {
		serviceNames[s.ID] = s.Name
	}
	checkMap := map[string]string{}
	for _, t := range a.adBoard(u.IsAdmin()).Teams {
		for _, cell := range t.Services {
			checkMap[strconv.Itoa(int(t.AccountID))+":"+strconv.Itoa(int(cell.ServiceID))] = cell.Status
		}
	}
	captain := u.IsAdmin()
	token := ""
	if u.TeamID != nil {
		var team models.Team
		if a.DB.First(&team, *u.TeamID).Error == nil {
			captain = captain || team.CaptainID != nil && *team.CaptainID == u.ID
			token = a.teamADToken(&team)
		}
	}
	a.HTML(c, http.StatusOK, "attack.html", gin.H{
		"Title": "Attack-Defense", "Round": round, "SecondsLeft": a.roundSecondsLeft(round),
		"Instances": insts, "Mine": mine, "Steals": steals, "Checks": checkMap, "ServiceNames": serviceNames,
		"Captain": captain, "Token": token, "FlagHint": a.flagHint(),
		"SubmitURL":  requestBase(c) + "/api/v1/ad/flags",
		"PageScript": "/static/js/attack.js?v=3", "Flashes": a.flashes(c),
	})
}

func (a *App) roundSecondsLeft(round *models.ADRound) int {
	if round == nil || round.Ended != nil {
		return 0
	}
	end := round.Started.Add(time.Duration(a.Store.ADRoundSeconds()) * time.Second)
	left := int(time.Until(end).Seconds())
	if left < 0 {
		return 0
	}
	return left
}

// flagHint is the submit placeholder: the configured prefix with the same closing
// brace rule the engine uses when it issues flags.
func (a *App) flagHint() string {
	prefix := a.Store.ADFlagPrefix()
	if strings.Contains(prefix, "{") && !strings.HasSuffix(prefix, "{") {
		return prefix + "..."
	}
	return prefix + "...}"
}

func requestBase(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

func (a *App) teamADToken(team *models.Team) string {
	if team.ADToken == "" {
		team.ADToken = webutil.RandomHex(20)
		a.DB.Model(team).Update("ad_token", team.ADToken)
	}
	return team.ADToken
}

func (a *App) AttackTokenPOST(c *gin.Context) {
	u := middleware.CurrentUser(c)
	var team models.Team
	if u == nil || u.TeamID == nil || a.DB.First(&team, *u.TeamID).Error != nil {
		c.Redirect(http.StatusFound, "/attack")
		return
	}
	if !u.IsAdmin() && (team.CaptainID == nil || *team.CaptainID != u.ID) {
		a.flash(c, "danger", "Only the team captain can change the token")
		c.Redirect(http.StatusFound, "/attack")
		return
	}
	team.ADToken = ""
	a.teamADToken(&team)
	a.flash(c, "success", "Team token changed")
	c.Redirect(http.StatusFound, "/attack")
}

const adMaxFlagsPerRequest = 500

type adFlagResult struct {
	Flag    string `json:"flag"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Points  int    `json:"points,omitempty"`
}

func adError(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"success": false, "errors": []string{msg}})
}

// adSubmitter resolves the team either from the X-Team-Token header (scripts)
// or from the browser session. Flag submission does not require a CSRF token:
// the session cookie is enough, and scripts authenticate with the team token.
func (a *App) adSubmitter(c *gin.Context) (*models.Team, uint, bool) {
	var team models.Team
	if tok := strings.TrimSpace(c.GetHeader("X-Team-Token")); tok != "" {
		if len(tok) < 32 || a.DB.Where("ad_token = ?", tok).First(&team).Error != nil {
			adError(c, http.StatusUnauthorized, "Invalid team token")
			return nil, 0, false
		}
		if team.Banned {
			adError(c, http.StatusForbidden, "Your team has been banned")
			return nil, 0, false
		}
		return &team, 0, true
	}
	u := middleware.CurrentUser(c)
	if u == nil {
		adError(c, http.StatusUnauthorized, "Authentication required")
		return nil, 0, false
	}
	if u.Banned {
		adError(c, http.StatusForbidden, "You have been banned")
		return nil, 0, false
	}
	if u.TeamID == nil || a.DB.First(&team, *u.TeamID).Error != nil {
		adError(c, http.StatusForbidden, "Join a team first")
		return nil, 0, false
	}
	if team.Banned {
		adError(c, http.StatusForbidden, "Your team has been banned")
		return nil, 0, false
	}
	return &team, u.ID, true
}

// readADFlags accepts {"submission": "..."} or a form field for one flag, and a
// JSON array, {"flags": [...]} or plain text (one flag per line) for many.
func readADFlags(c *gin.Context) ([]string, bool) {
	var flags []string
	batch := false
	switch c.ContentType() {
	case "application/x-www-form-urlencoded", "multipart/form-data":
		flags = []string{c.PostForm("submission")}
	default:
		raw, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
		raw = bytes.TrimSpace(raw)
		switch {
		case len(raw) > 0 && raw[0] == '[':
			_ = json.Unmarshal(raw, &flags)
			batch = true
		case len(raw) > 0 && raw[0] == '{':
			var body struct {
				Submission string   `json:"submission"`
				Flag       string   `json:"flag"`
				Flags      []string `json:"flags"`
			}
			_ = json.Unmarshal(raw, &body)
			if body.Flags != nil {
				flags, batch = body.Flags, true
			} else if body.Submission != "" {
				flags = []string{body.Submission}
			} else {
				flags = []string{body.Flag}
			}
		default:
			flags = strings.Fields(string(raw))
			batch = true
		}
	}
	seen := map[string]bool{}
	out := flags[:0]
	for _, f := range flags {
		f = strings.TrimSpace(f)
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out, batch
}

func (a *App) AttackSubmit(c *gin.Context) {
	team, userID, ok := a.adSubmitter(c)
	if !ok {
		return
	}
	if !a.Store.IsAD() || !a.Store.DuringCTF() {
		adError(c, http.StatusForbidden, "Attack-Defense is not active")
		return
	}
	if a.Store.Preparing() {
		adError(c, http.StatusForbidden, "Preparation is in progress")
		return
	}
	if !a.ADLimiter.Allow("ad:" + strconv.Itoa(int(team.ID))) {
		adError(c, http.StatusTooManyRequests, "Too many submissions")
		return
	}
	flags, batch := readADFlags(c)
	if len(flags) == 0 {
		adError(c, http.StatusBadRequest, "Empty flag")
		return
	}
	if len(flags) > adMaxFlagsPerRequest {
		adError(c, http.StatusBadRequest, "Too many flags in one request (max 500)")
		return
	}
	results := make([]adFlagResult, 0, len(flags))
	for _, f := range flags {
		results = append(results, a.submitADFlag(team.ID, userID, f))
	}
	if batch {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": results})
		return
	}
	res := results[0]
	if res.Status == "own_flag" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "errors": []string{res.Message}, "data": res})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": res})
}

func (a *App) submitADFlag(teamID, userID uint, flag string) adFlagResult {
	res := adFlagResult{Flag: flag, Status: "incorrect", Message: "Invalid flag"}
	fl, ok := a.Flags.ByValue(flag)
	if !ok {
		return res
	}
	if fl.TeamID == teamID {
		res.Status, res.Message = "own_flag", "Cannot submit own flag"
		return res
	}
	if time.Now().After(fl.Expires) {
		res.Message = "Expired flag"
		return res
	}
	if fl.PutStatus != models.CheckOK {
		return res
	}
	var svc models.ADService
	if a.DB.First(&svc, fl.ServiceID).Error != nil || !svc.Active {
		res.Message = "Unknown service"
		return res
	}
	steal := models.ADSteal{
		FlagID: fl.ID, AttackerTeamID: teamID, VictimTeamID: fl.TeamID,
		ServiceID: fl.ServiceID, RoundID: fl.RoundID, Points: svc.AttackReward,
		UserID: userID, Date: time.Now(),
	}
	if a.DB.Create(&steal).Error != nil {
		res.Status, res.Message = "already_solved", "Already submitted"
		return res
	}
	a.dropADCache()
	res.Status, res.Message, res.Points = "correct", "Flag accepted", svc.AttackReward
	return res
}

func (a *App) AttackHostsPOST(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil || u.TeamID == nil {
		c.Redirect(http.StatusFound, "/attack")
		return
	}
	var team models.Team
	if a.DB.First(&team, *u.TeamID).Error != nil {
		c.Redirect(http.StatusFound, "/attack")
		return
	}
	if !u.IsAdmin() && (team.CaptainID == nil || *team.CaptainID != u.ID) {
		a.flash(c, "danger", "Only the team captain can set vulnbox hosts")
		c.Redirect(http.StatusFound, "/attack")
		return
	}
	_ = ad.EnsureInstances(a.DB)
	var insts []models.ADInstance
	a.DB.Where("team_id = ?", team.ID).Find(&insts)
	for _, inst := range insts {
		host := strings.TrimSpace(c.PostForm("host_" + strconv.Itoa(int(inst.ID))))
		portStr := c.PostForm("port_" + strconv.Itoa(int(inst.ID)))
		port, _ := strconv.Atoi(portStr)
		updates := map[string]any{"host": host}
		if port > 0 {
			updates["port"] = port
		}
		a.DB.Model(&inst).Updates(updates)
	}
	a.flash(c, "success", "Vulnbox addresses saved")
	c.Redirect(http.StatusFound, "/attack")
}

// canSeeADDashboard keeps the AD dashboard public, including anonymous visitors,
// unless the admin hid scores completely.
func (a *App) canSeeADDashboard(u *models.User) bool {
	switch a.Store.ScoreVisibility() {
	case models.VisibilityHidden, models.VisibilityAdminsOnly:
		return u != nil && u.IsAdmin()
	default:
		return true
	}
}

const adCacheTTL = 2 * time.Second

type adBoardCache struct {
	mu    sync.Mutex
	at    [2]time.Time
	board [2]scoring.ADBoard
}

// adBoard caches the scoreboard for a short time: every open dashboard polls it.
func (a *App) adBoard(admin bool) scoring.ADBoard {
	i := 0
	if admin {
		i = 1
	}
	a.adCache.mu.Lock()
	defer a.adCache.mu.Unlock()
	if time.Since(a.adCache.at[i]) > adCacheTTL {
		var defended map[[2]uint]int
		if a.Flags != nil {
			defended = a.Flags.Defended()
		}
		a.adCache.board[i] = scoring.ADDashboard(a.DB, defended, admin)
		a.adCache.at[i] = time.Now()
	}
	return a.adCache.board[i]
}

func (a *App) dropADCache() {
	a.adCache.mu.Lock()
	a.adCache.at = [2]time.Time{}
	a.adCache.mu.Unlock()
}

func (a *App) APIADScoreboard(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.Store.IsAD() || !a.canSeeScores(u) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"Forbidden"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": a.adBoard(u != nil && u.IsAdmin()).Teams})
}

func (a *App) DashboardPage(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.Store.IsAD() {
		a.HTML(c, http.StatusNotFound, "error.html", gin.H{"Title": "Dashboard", "Message": "Attack-Defense is not enabled."})
		return
	}
	if !a.canSeeADDashboard(u) {
		a.HTML(c, http.StatusForbidden, "error.html", gin.H{"Title": "Dashboard", "Message": "Scoreboard is not available."})
		return
	}
	a.HTML(c, http.StatusOK, "dashboard.html", gin.H{
		"Title": "Dashboard", "Board": a.dashboardData(u),
		"PageScript": "/static/js/dashboard.js?v=4", "Flashes": a.flashes(c),
	})
}

func (a *App) APIADDashboard(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if !a.Store.IsAD() || !a.canSeeADDashboard(u) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"Forbidden"}})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"success": true, "data": a.dashboardData(u)})
}

func (a *App) roundInfo() gin.H {
	info := gin.H{"number": 0, "seconds_left": 0, "round_seconds": a.Store.ADRoundSeconds()}
	if r := ad.CurrentRound(a.DB); r != nil {
		info["number"] = r.Number
		info["seconds_left"] = a.roundSecondsLeft(r)
	}
	return info
}

func (a *App) dashboardData(u *models.User) gin.H {
	admin := u != nil && u.IsAdmin()
	var mine uint
	if u != nil && u.TeamID != nil {
		mine = *u.TeamID
	}
	board := a.adBoard(admin)
	teams := make([]scoring.ADStanding, len(board.Teams))
	copy(teams, board.Teams)
	if !admin {
		for i := range teams {
			if teams[i].AccountID == mine {
				continue
			}
			cells := make([]scoring.ADServiceCell, len(teams[i].Services))
			copy(cells, teams[i].Services)
			for j := range cells {
				cells[j].Message = ""
			}
			teams[i].Services = cells
		}
	}
	return gin.H{
		"round": a.roundInfo(), "paused": a.Store.ADPaused(), "during": a.Store.DuringCTF(),
		"preparing": a.Store.Preparing(), "prep_seconds_left": a.Store.PrepSecondsLeft(),
		"services": board.Services, "teams": teams, "me": mine,
	}
}

func (a *App) APIADStatus(c *gin.Context) {
	if !a.Store.IsAD() {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "errors": []string{"Attack-Defense is not enabled"}})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"round": a.roundInfo(), "paused": a.Store.ADPaused(), "during": a.Store.DuringCTF(),
		"round_seconds": a.Store.ADRoundSeconds(),
	}})
}

func (a *App) AdminAD(c *gin.Context) {
	_ = ad.EnsureInstances(a.DB)
	var services []models.ADService
	a.DB.Order("id").Find(&services)
	var teams []models.Team
	a.DB.Order("name").Find(&teams)
	var insts []models.ADInstance
	a.DB.Preload("Team").Preload("Service").Find(&insts)
	round := ad.CurrentRound(a.DB)
	var checks []models.ADCheck
	if round != nil {
		a.DB.Where("round_id = ?", round.ID).Order("id DESC").Find(&checks)
	}
	var steals []models.ADSteal
	a.DB.Preload("Attacker").Preload("Victim").Order("date DESC").Limit(50).Find(&steals)
	var flags []models.ADFlag
	if a.Flags != nil {
		flags = a.Flags.Recent(400)
	}
	latest := map[string]models.ADFlag{}
	for _, fl := range flags {
		key := strconv.Itoa(int(fl.TeamID)) + ":" + strconv.Itoa(int(fl.ServiceID))
		if _, ok := latest[key]; !ok {
			latest[key] = fl
		}
	}
	type hostRow struct {
		models.ADInstance
		PutStatus  string
		PutMessage string
	}
	rows := make([]hostRow, 0, len(insts))
	for _, inst := range insts {
		row := hostRow{ADInstance: inst}
		if fl, ok := latest[strconv.Itoa(int(inst.TeamID))+":"+strconv.Itoa(int(inst.ServiceID))]; ok {
			row.PutStatus = fl.PutStatus
			row.PutMessage = fl.PutMessage
		}
		rows = append(rows, row)
	}
	teamNames := map[uint]string{}
	for _, t := range teams {
		teamNames[t.ID] = t.Name
	}
	serviceNames := map[uint]string{}
	for _, s := range services {
		serviceNames[s.ID] = s.Name
	}
	a.HTML(c, http.StatusOK, "admin_ad.html", gin.H{
		"Title": "Attack-Defense", "Admin": true, "Services": services, "Teams": teams,
		"Instances": insts, "Hosts": rows, "Round": round, "Checks": checks, "Steals": steals,
		"TeamNames": teamNames, "ServiceNames": serviceNames,
		"Paused": a.Store.ADPaused(), "RoundSeconds": a.Store.ADRoundSeconds(),
		"FlagLife": a.Store.ADFlagLifetime(), "FlagPrefix": a.Store.ADFlagPrefix(),
		"PrepStart": clockInput(a.Store.ADPrepStart(), a.Store.Location()), "PrepMinutes": a.Store.ADPrepMinutes(),
		"Flashes": a.flashes(c),
	})
}

func (a *App) AdminADConfigPOST(c *gin.Context) {
	paused := "false"
	if c.PostForm("ad_paused") == "on" {
		paused = "true"
	}
	_ = a.Store.SetMany(map[string]string{
		"ad_paused":               paused,
		"ad_round_seconds":        c.PostForm("ad_round_seconds"),
		"ad_flag_lifetime_rounds": c.PostForm("ad_flag_lifetime_rounds"),
		"ad_flag_prefix":          c.PostForm("ad_flag_prefix"),
		"ad_prep_start":           clockValue(c.PostForm("ad_prep_start"), a.Store.Location()),
		"ad_prep_minutes":         c.PostForm("ad_prep_minutes"),
	})
	a.flash(c, "success", "Attack-Defense settings saved")
	c.Redirect(http.StatusFound, "/admin/ad")
}

func clockInput(t *time.Time, loc *time.Location) string {
	if t == nil {
		return ""
	}
	if loc == nil {
		loc = time.UTC
	}
	return t.In(loc).Format("2006-01-02T15:04")
}

func clockValue(raw string, loc *time.Location) string {
	t := parseClock(raw, loc)
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func parseClock(raw string, loc *time.Location) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if loc == nil {
		loc = time.UTC
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &t
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return &t
		}
	}
	return nil
}

func loadZone(name string) (*time.Location, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "UTC"
	}
	loc, err := time.LoadLocation(name)
	return loc, name, err
}

func formInt(c *gin.Context, key string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(c.PostForm(key)))
	if err != nil || n < 0 {
		return def
	}
	return n
}

func (a *App) AdminADServicePOST(c *gin.Context) {
	port, _ := strconv.Atoi(c.PostForm("port"))
	ar := formInt(c, "attack_reward", models.ADDefaultAttackReward)
	sr := formInt(c, "sla_reward", models.ADDefaultSLAReward)
	dp := formInt(c, "defense_penalty", models.ADDefaultDefensePenalty)
	dr := formInt(c, "defense_reward", models.ADDefaultDefenseReward)
	down := formInt(c, "down_penalty", models.ADDefaultDownPenalty)
	svc := models.ADService{
		Name: strings.TrimSpace(c.PostForm("name")), Description: c.PostForm("description"),
		Port: port, CheckerURL: strings.TrimSpace(c.PostForm("checker_url")),
		AttackReward: ar, SLAReward: sr, DefensePenalty: dp, DefenseReward: dr, DownPenalty: down, Active: c.PostForm("active") != "off",
	}
	if svc.Name == "" {
		a.flash(c, "danger", "Service name is required")
		c.Redirect(http.StatusFound, "/admin/ad")
		return
	}
	if id, _ := strconv.Atoi(c.PostForm("id")); id > 0 {
		svc.ID = uint(id)
		a.DB.Model(&models.ADService{}).Where("id = ?", id).Updates(map[string]any{
			"name": svc.Name, "description": svc.Description, "port": svc.Port,
			"checker_url": svc.CheckerURL, "attack_reward": svc.AttackReward,
			"sla_reward": svc.SLAReward, "defense_penalty": svc.DefensePenalty, "defense_reward": svc.DefenseReward,
			"down_penalty": svc.DownPenalty, "active": c.PostForm("active") == "on",
		})
		a.flash(c, "success", "Service updated")
	} else {
		a.DB.Create(&svc)
		a.DB.Model(&svc).Updates(map[string]any{
			"attack_reward": ar, "sla_reward": sr, "defense_penalty": dp, "defense_reward": dr, "down_penalty": down,
		})
		a.flash(c, "success", "Service created")
	}
	_ = ad.EnsureInstances(a.DB)
	c.Redirect(http.StatusFound, "/admin/ad")
}

func (a *App) AdminADPointsPOST(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var svc models.ADService
	if a.DB.First(&svc, id).Error != nil {
		a.flash(c, "danger", "Service not found")
		c.Redirect(http.StatusFound, "/admin/ad")
		return
	}
	a.DB.Model(&svc).Updates(map[string]any{
		"attack_reward":   formInt(c, "attack_reward", svc.AttackReward),
		"defense_penalty": formInt(c, "defense_penalty", svc.DefensePenalty),
		"defense_reward":  formInt(c, "defense_reward", svc.DefenseReward),
		"sla_reward":      formInt(c, "sla_reward", svc.SLAReward),
		"down_penalty":    formInt(c, "down_penalty", svc.DownPenalty),
	})
	a.dropADCache()
	a.flash(c, "success", "Scoring saved")
	c.Redirect(http.StatusFound, "/admin/ad")
}

func (a *App) AdminADServiceDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var svc models.ADService
	if a.DB.First(&svc, id).Error == nil && svc.ComposeDir != "" && dirExists(svc.ComposeDir) {
		_, _ = tasks.DownVolumes(svc.ComposeDir)
	}
	a.DB.Where("service_id = ?", id).Delete(&models.ADInstance{})
	if a.Flags != nil {
		a.Flags.DeleteService(uint(id))
	}
	a.DB.Where("service_id = ?", id).Delete(&models.ADCheck{})
	a.DB.Where("service_id = ?", id).Delete(&models.ADSteal{})
	a.DB.Delete(&models.ADService{}, id)
	a.flash(c, "success", "Service deleted")
	c.Redirect(http.StatusFound, "/admin/ad")
}

func (a *App) AdminADHostsPOST(c *gin.Context) {
	_ = ad.EnsureInstances(a.DB)
	var insts []models.ADInstance
	a.DB.Find(&insts)
	for _, inst := range insts {
		key := strconv.Itoa(int(inst.ID))
		host := strings.TrimSpace(c.PostForm("host_" + key))
		port, _ := strconv.Atoi(c.PostForm("port_" + key))
		upd := map[string]any{"host": host}
		if port > 0 {
			upd["port"] = port
		}
		a.DB.Model(&inst).Updates(upd)
	}
	a.flash(c, "success", "Team hosts saved")
	c.Redirect(http.StatusFound, "/admin/ad")
}
