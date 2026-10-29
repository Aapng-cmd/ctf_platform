package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/config"
	"github.com/aapng-cmd/ctf_platform/internal/flags"
	"github.com/aapng-cmd/ctf_platform/internal/middleware"
	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/store"
	"github.com/aapng-cmd/ctf_platform/internal/webutil"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type App struct {
	DB          *gorm.DB
	Store       *store.Store
	Flags       flags.Store
	Cfg         config.Config
	Limiter     *webutil.RateLimiter
	AuthLimiter *webutil.RateLimiter
	ADLimiter   *webutil.RateLimiter
	tmpl        *template.Template
	adCache     adBoardCache
}

func New(db *gorm.DB, st *store.Store, cfg config.Config) (*App, error) {
	a := &App{
		DB:      db,
		Store:   st,
		Cfg:     cfg,
		Limiter: webutil.NewRateLimiter(time.Minute, 10),
		// Login, registration and team passwords. Keyed by client and account so one NAT is not a single bucket.
		AuthLimiter: webutil.NewRateLimiter(time.Minute, 30),
		// Exploits submit every round; batches of up to 500 flags count as one request.
		ADLimiter: webutil.NewRateLimiter(time.Minute, 120),
	}
	if err := a.LoadTemplates(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) LoadTemplates() error {
	funcs := template.FuncMap{
		"md": webutil.Markdown,
		"cfg": func(key string) string {
			return a.Store.Get(key)
		},
		"ctfName":  a.Store.CTFName,
		"userMode": a.Store.UserMode,
		"isTeams":  a.Store.IsTeams,
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.In(a.Store.Location()).Format("02.01.2006 15:04:05")
		},
		"formatDate": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.In(a.Store.Location()).Format("02.01.2006 15:04")
		},
		"timezone":    a.Store.Timezone,
		"zoneChoices": zoneChoices,
		"tzCity":      tzCityByID,
		"add":         func(a, b int) int { return a + b },
		"sub":         func(a, b int) int { return a - b },
		"mul":         func(a, b int) int { return a * b },
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
		"eq":       func(a, b any) bool { return a == b },
		"has":      func(m map[uint]bool, id uint) bool { return m[id] },
		"adKey":    func(tid, sid uint) string { return fmt.Sprintf("%d:%d", tid, sid) },
		"idEq":     func(id *uint, other uint) bool { return id != nil && *id == other },
		"lower":    strings.ToLower,
		"trim":     strings.TrimSpace,
		"split":    strings.Split,
		"contains": strings.Contains,
		"dict": func(values ...any) map[string]any {
			m := map[string]any{}
			for i := 0; i+1 < len(values); i += 2 {
				k, _ := values[i].(string)
				m[k] = values[i+1]
			}
			return m
		},
		"countryName": func(code string) string {
			for _, c := range webutil.Countries {
				if c.Code == code {
					return c.Name
				}
			}
			return code
		},
	}
	t, err := template.New("").Funcs(funcs).ParseGlob(filepath.Join("web", "templates", "*.html"))
	if err != nil {
		return err
	}
	if extra, err := t.ParseGlob(filepath.Join("web", "templates", "admin", "*.html")); err == nil {
		t = extra
	}
	a.tmpl = t
	return nil
}

func (a *App) HTML(c *gin.Context, code int, name string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}
	u := middleware.CurrentUser(c)
	data["User"] = u
	data["CSRF"] = c.GetString("csrf")
	data["CTFName"] = a.Store.CTFName()
	data["UserMode"] = a.Store.UserMode()
	data["IsTeams"] = a.Store.IsTeams()
	data["IsAD"] = a.Store.IsAD()
	data["IsJeopardy"] = a.Store.IsJeopardy()
	data["GameMode"] = a.Store.GameMode()
	data["Setup"] = a.Store.SetupComplete()
	data["Started"] = a.Store.Started()
	data["Ended"] = a.Store.Ended()
	data["During"] = a.Store.DuringCTF()
	data["RequestPath"] = c.Request.URL.Path
	data["Countries"] = webutil.Countries
	if u != nil {
		var n int64
		a.DB.Model(&models.Notification{}).Count(&n)
		data["NotificationCount"] = n
	}
	var pages []models.Page
	a.DB.Where("draft = ? AND hidden = ? AND route <> ?", false, false, "index").Order("id").Find(&pages)
	data["NavPages"] = pages
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := a.tmpl.ExecuteTemplate(c.Writer, name, data); err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
	}
}

func (a *App) flash(c *gin.Context, kind, msg string) {
	s := sessions.Default(c)
	s.AddFlash(kind + "|" + msg)
	_ = s.Save()
}

func (a *App) flashes(c *gin.Context) []gin.H {
	s := sessions.Default(c)
	raw := s.Flashes()
	_ = s.Save()
	var out []gin.H
	for _, f := range raw {
		str, _ := f.(string)
		kind, msg, _ := strings.Cut(str, "|")
		out = append(out, gin.H{"Type": kind, "Message": msg})
	}
	return out
}

func (a *App) canSeeChallenges(u *models.User) bool {
	vis := a.Store.ChallengeVisibility()
	switch vis {
	case models.VisibilityPublic:
		return true
	case models.VisibilityPrivate:
		return u != nil
	case models.VisibilityAdminsOnly:
		return u != nil && u.IsAdmin()
	default:
		return u != nil
	}
}

func (a *App) canSeeScores(u *models.User) bool {
	vis := a.Store.ScoreVisibility()
	switch vis {
	case models.VisibilityPublic:
		return true
	case models.VisibilityPrivate:
		return u != nil
	case models.VisibilityHidden, models.VisibilityAdminsOnly:
		return u != nil && u.IsAdmin()
	default:
		return true
	}
}

func (a *App) canSeeAccounts(u *models.User) bool {
	vis := a.Store.AccountVisibility()
	switch vis {
	case models.VisibilityPublic:
		return true
	case models.VisibilityPrivate:
		return u != nil
	case models.VisibilityAdminsOnly:
		return u != nil && u.IsAdmin()
	default:
		return true
	}
}

func (a *App) freezeFor(u *models.User) *time.Time {
	if u != nil && u.IsAdmin() {
		return nil
	}
	return a.Store.Freeze()
}

func (a *App) accountID(u *models.User) uint {
	if a.Store.IsTeams() && u.TeamID != nil {
		return *u.TeamID
	}
	return u.ID
}

func wantsJSON(c *gin.Context) bool {
	return strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.Contains(c.GetHeader("Accept"), "application/json")
}
