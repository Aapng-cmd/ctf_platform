package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/store"
	"github.com/aapng-cmd/ctf_platform/internal/webutil"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const UserKey = "current_user"

func CurrentUser(c *gin.Context) *models.User {
	v, ok := c.Get(UserKey)
	if !ok || v == nil {
		return nil
	}
	u, _ := v.(*models.User)
	return u
}

func LoadUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s := sessions.Default(c)
		id := asUint(s.Get("user_id"))
		if id != 0 {
			var u models.User
			if err := db.Preload("Team").First(&u, id).Error; err == nil {
				c.Set(UserKey, &u)
			}
		}
		c.Next()
	}
}

func SetupGuard(cfg *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/setup") || strings.HasPrefix(path, "/static") || strings.HasPrefix(path, "/swagger") {
			c.Next()
			return
		}
		if !cfg.SetupComplete() {
			c.Redirect(http.StatusFound, "/setup")
			c.Abort()
			return
		}
		c.Next()
	}
}

func Authed() gin.HandlerFunc {
	return func(c *gin.Context) {
		if CurrentUser(c) == nil {
			if wantsJSON(c) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "errors": []string{"Authentication required"}})
				return
			}
			c.Redirect(http.StatusFound, "/login?next="+url.QueryEscape(webutil.SafeNext(c.Request.URL.RequestURI(), "/challenges")))
			c.Abort()
			return
		}
		c.Next()
	}
}

func Admin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil || !u.IsAdmin() {
			if wantsJSON(c) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"Admins only"}})
				return
			}
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

func NotBanned() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u != nil && u.Banned {
			if wantsJSON(c) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"You have been banned"}})
				return
			}
			c.HTML(http.StatusForbidden, "error.html", gin.H{"title": "Banned", "message": "You have been banned from this CTF."})
			c.Abort()
			return
		}
		c.Next()
	}
}

func LocalhostOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		// A reverse proxy on loopback would make every client look local.
		// Forwarding headers mean the peer is a proxy, so Swagger stays closed.
		if c.GetHeader("X-Forwarded-For") != "" || c.GetHeader("X-Real-IP") != "" || c.GetHeader("Forwarded") != "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"errors":  []string{"API documentation is only available from localhost"},
			})
			return
		}
		// Use RemoteAddr only. ClientIP() can be spoofed via X-Forwarded-For
		// unless trusted proxies are disabled; this stays locked to the real peer.
		if webutil.IsLoopback(c.Request.RemoteAddr) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"success": false,
			"errors":  []string{"API documentation is only available from localhost"},
		})
	}
}

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		c.Next()
	}
}

func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		s := sessions.Default(c)
		token, _ := s.Get("csrf").(string)
		if token == "" {
			token = webutil.RandomHex(16)
			s.Set("csrf", token)
			_ = s.Save()
		}
		c.Set("csrf", token)
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch || c.Request.Method == http.MethodDelete {
			// Scripts submit Attack-Defense flags with X-Team-Token and no cookie.
			// A cross-site page cannot add that header, so this request is not CSRF.
			// Cookie-authenticated API calls still need the CSRF token.
			if strings.TrimSpace(c.GetHeader("X-Team-Token")) != "" {
				c.Next()
				return
			}
			got := c.PostForm("nonce")
			if got == "" {
				got = c.GetHeader("CSRF-Token")
			}
			if got != token {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "errors": []string{"Invalid CSRF token"}})
				return
			}
		}
		c.Next()
	}
}

func asUint(v any) uint {
	switch n := v.(type) {
	case uint:
		return n
	case int:
		return uint(n)
	case int64:
		return uint(n)
	case float64:
		return uint(n)
	default:
		return 0
	}
}

func wantsJSON(c *gin.Context) bool {
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		return true
	}
	accept := c.GetHeader("Accept")
	return strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html")
}
