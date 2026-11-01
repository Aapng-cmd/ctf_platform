package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/middleware"
	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/webutil"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func (a *App) SetupGET(c *gin.Context) {
	if a.setupClosed() {
		a.HTML(c, http.StatusNotFound, "error.html", gin.H{"Title": "404", "Message": "Page not found"})
		return
	}
	a.HTML(c, http.StatusOK, "setup.html", gin.H{"Title": "Setup", "Flashes": a.flashes(c)})
}

var setupMu sync.Mutex

func (a *App) SetupPOST(c *gin.Context) {
	setupMu.Lock()
	defer setupMu.Unlock()
	if a.setupClosed() {
		a.HTML(c, http.StatusNotFound, "error.html", gin.H{"Title": "404", "Message": "Page not found"})
		return
	}
	name := strings.TrimSpace(c.PostForm("ctf_name"))
	desc := c.PostForm("ctf_description")
	adminName := strings.TrimSpace(c.PostForm("name"))
	adminEmail := strings.TrimSpace(c.PostForm("email"))
	adminPass := c.PostForm("password")
	if name == "" || !webutil.ValidName(adminName) || !webutil.ValidEmail(adminEmail) || len(adminPass) < 6 {
		a.flash(c, "danger", "Please fill all required fields. Password must be at least 6 characters.")
		c.Redirect(http.StatusFound, "/setup")
		return
	}
	_, zone, zerr := loadZone(c.PostForm("timezone"))
	if zerr != nil {
		a.flash(c, "danger", "Unknown timezone")
		c.Redirect(http.StatusFound, "/setup")
		return
	}
	hash, err := webutil.HashPassword(adminPass)
	if err != nil {
		a.flash(c, "danger", "Could not hash password")
		c.Redirect(http.StatusFound, "/setup")
		return
	}
	admin := models.User{
		Email:    adminEmail,
		Name:     adminName,
		Password: hash,
		Type:     models.UserTypeAdmin,
		Verified: true,
		Created:  time.Now(),
	}
	if err := a.DB.Create(&admin).Error; err != nil {
		a.flash(c, "danger", "Could not create admin user (name or email already used)")
		c.Redirect(http.StatusFound, "/setup")
		return
	}
	index := models.Page{Route: "index"}
	content := defaultIndex(name, desc)
	if err := a.DB.Where(&models.Page{Route: "index"}).First(&index).Error; err != nil {
		index = models.Page{Title: name, Route: "index", Content: content}
		_ = a.DB.Create(&index).Error
	} else {
		index.Title = name
		index.Content = content
		_ = a.DB.Save(&index).Error
	}
	_ = a.Store.SetMany(map[string]string{
		"ctf_name":                name,
		"ctf_description":         desc,
		"user_mode":               models.ModeTeams,
		"game_mode":               models.GameMixed,
		"timezone":                zone,
		"ad_running":              "false",
		"ad_event_id":             "0",
		"ad_paused":               "true",
		"ad_round_seconds":        "15",
		"ad_flag_lifetime_rounds": "3",
		"ad_flag_prefix":          "FLAG{",
		"setup":                   "true",
		"challenge_visibility":    models.VisibilityPrivate,
		"score_visibility":        models.VisibilityPublic,
		"account_visibility":      models.VisibilityPublic,
		"registration_visibility": models.VisibilityPublic,
		"start":                   "",
		"end":                     "",
		"verify_emails":           "false",
		"max_team_size":           "0",
		"theme":                   "core",
	})
	s := sessions.Default(c)
	s.Set("user_id", int(admin.ID))
	_ = s.Save()
	c.Redirect(http.StatusFound, "/admin/config")
}

// setupClosed is true once the first admin exists or setup has been saved.
// /setup then answers 404. Later admins are created in the admin panel.
func (a *App) setupClosed() bool {
	if a.Store != nil && a.Store.SetupComplete() {
		return true
	}
	var n int64
	a.DB.Model(&models.User{}).Where("type = ?", models.UserTypeAdmin).Count(&n)
	return n > 0
}

func defaultIndex(name, desc string) string {
	if desc == "" {
		desc = "Платформа для соревнований Capture The Flag."
	}
	return "# " + name + "\n\n" + desc + "\n\nЗарегистрируйтесь и начинайте решать задания."
}

func (a *App) LoginGET(c *gin.Context) {
	if middleware.CurrentUser(c) != nil {
		c.Redirect(http.StatusFound, "/challenges")
		return
	}
	a.HTML(c, http.StatusOK, "login.html", gin.H{"Title": "Login", "Next": webutil.SafeNext(c.Query("next"), ""), "Flashes": a.flashes(c)})
}

func (a *App) LoginPOST(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	password := c.PostForm("password")
	if !a.AuthLimiter.Allow("login:" + c.ClientIP() + ":" + strings.ToLower(name)) {
		a.flash(c, "danger", "Too many attempts. Wait a minute and try again.")
		c.Redirect(http.StatusFound, "/login")
		return
	}
	var u models.User
	err := a.DB.Where("name = ? OR email = ?", name, name).First(&u).Error
	if !webutil.CheckPassword(u.Password, password) || err != nil {
		a.flash(c, "danger", "Your username or password is incorrect")
		c.Redirect(http.StatusFound, "/login")
		return
	}
	if u.Banned {
		a.flash(c, "danger", "You have been banned")
		c.Redirect(http.StatusFound, "/login")
		return
	}
	s := sessions.Default(c)
	s.Set("user_id", int(u.ID))
	_ = s.Save()
	next := webutil.SafeNext(c.PostForm("next"), "/challenges")
	if next == "" {
		next = "/challenges"
	}
	c.Redirect(http.StatusFound, next)
}

func (a *App) RegisterGET(c *gin.Context) {
	if middleware.CurrentUser(c) != nil {
		c.Redirect(http.StatusFound, "/challenges")
		return
	}
	if a.Store.RegistrationVisibility() != models.VisibilityPublic {
		a.HTML(c, http.StatusForbidden, "error.html", gin.H{"Title": "Registration", "Message": "Registration is currently disabled."})
		return
	}
	a.HTML(c, http.StatusOK, "register.html", gin.H{"Title": "Register", "Flashes": a.flashes(c)})
}

func (a *App) RegisterPOST(c *gin.Context) {
	if a.Store.RegistrationVisibility() != models.VisibilityPublic {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	email := strings.TrimSpace(c.PostForm("email"))
	password := c.PostForm("password")
	if !a.AuthLimiter.Allow("register:" + c.ClientIP()) {
		a.flash(c, "danger", "Too many attempts. Wait a minute and try again.")
		c.Redirect(http.StatusFound, "/register")
		return
	}
	website := strings.TrimSpace(c.PostForm("website"))
	affiliation := strings.TrimSpace(c.PostForm("affiliation"))
	country := strings.TrimSpace(c.PostForm("country"))
	if !webutil.ValidName(name) || !webutil.ValidEmail(email) || len(password) < 6 {
		a.flash(c, "danger", "Invalid registration details. Password must be at least 6 characters.")
		c.Redirect(http.StatusFound, "/register")
		return
	}
	hash, err := webutil.HashPassword(password)
	if err != nil {
		a.flash(c, "danger", "Could not create account")
		c.Redirect(http.StatusFound, "/register")
		return
	}
	u := models.User{
		Email: email, Name: name, Password: hash, Type: models.UserTypeUser,
		Website: website, Affiliation: affiliation, Country: country,
		Verified: true, Created: time.Now(),
	}
	if err := a.DB.Create(&u).Error; err != nil {
		a.flash(c, "danger", "That name or email is already taken")
		c.Redirect(http.StatusFound, "/register")
		return
	}
	s := sessions.Default(c)
	s.Set("user_id", int(u.ID))
	_ = s.Save()
	c.Redirect(http.StatusFound, "/challenges")
}

func (a *App) Logout(c *gin.Context) {
	s := sessions.Default(c)
	s.Clear()
	_ = s.Save()
	c.Redirect(http.StatusFound, "/")
}

func (a *App) SettingsGET(c *gin.Context) {
	a.HTML(c, http.StatusOK, "settings.html", gin.H{
		"Title": "Settings", "PageScript": "/static/js/avatar.js?v=1", "Flashes": a.flashes(c),
	})
}

func (a *App) SettingsPOST(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	if !webutil.ValidName(name) {
		a.flash(c, "danger", "Login is required")
		c.Redirect(http.StatusFound, "/settings")
		return
	}
	email := strings.TrimSpace(c.PostForm("email"))
	nameChanged := name != u.Name
	emailChanged := webutil.ValidEmail(email) && email != u.Email
	password := c.PostForm("password")
	if nameChanged || emailChanged || password != "" {
		if !webutil.CheckPassword(u.Password, c.PostForm("confirm")) {
			a.flash(c, "danger", "Current password is incorrect")
			c.Redirect(http.StatusFound, "/settings")
			return
		}
	}
	updates := map[string]any{"name": name}
	if emailChanged {
		updates["email"] = email
	}
	if password != "" {
		if len(password) < 6 {
			a.flash(c, "danger", "Password must be at least 6 characters")
			c.Redirect(http.StatusFound, "/settings")
			return
		}
		hash, err := webutil.HashPassword(password)
		if err != nil {
			a.flash(c, "danger", "Could not update profile")
			c.Redirect(http.StatusFound, "/settings")
			return
		}
		updates["password"] = hash
	}
	if err := a.DB.Model(u).Updates(updates).Error; err != nil {
		a.flash(c, "danger", "That name or email is already taken")
		c.Redirect(http.StatusFound, "/settings")
		return
	}
	if err := a.saveAvatar(c, u.ID); err != nil {
		a.flash(c, "danger", err.Error())
		c.Redirect(http.StatusFound, "/settings")
		return
	}
	a.flash(c, "success", "Your profile has been updated")
	c.Redirect(http.StatusFound, "/users/"+strconv.Itoa(int(u.ID)))
}
