package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/aapng-cmd/ctf_platform/docs"
	"github.com/aapng-cmd/ctf_platform/internal/ad"
	"github.com/aapng-cmd/ctf_platform/internal/config"
	"github.com/aapng-cmd/ctf_platform/internal/db"
	"github.com/aapng-cmd/ctf_platform/internal/flags"
	"github.com/aapng-cmd/ctf_platform/internal/handlers"
	"github.com/aapng-cmd/ctf_platform/internal/middleware"
	"github.com/aapng-cmd/ctf_platform/internal/store"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title CTF Platform API
// @version 1.0
// @description CTFd-compatible REST API. Swagger UI is available only from localhost.
// @BasePath /
func main() {
	cfg := config.Load()
	_ = os.MkdirAll(cfg.UploadDir, 0o755)
	_ = os.MkdirAll(cfg.DataDir, 0o755)

	gdb, err := db.Open(cfg)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := db.Migrate(gdb); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	st := store.New(gdb)
	flagStore, err := flags.Open(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	eng := ad.New(gdb, st, flagStore)
	eng.Start()
	defer eng.Stop()
	app, err := handlers.New(gdb, st, cfg)
	if err != nil {
		log.Fatalf("app: %v", err)
	}
	app.Flags = flagStore
	go app.RestoreDeployedTasks()

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	if len(cfg.TrustedProxies) == 0 {
		_ = r.SetTrustedProxies(nil)
	} else if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Fatalf("trusted proxies: %v", err)
	}
	r.Use(gin.Logger(), gin.Recovery(), middleware.SecurityHeaders())
	r.Use(gzip.Gzip(gzip.DefaultCompression,
		gzip.WithExcludedExtensions([]string{".woff2", ".woff", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".zip", ".gz"}),
		gzip.WithExcludedPathsRegexs([]string{`^/files/`, `^/swagger/`}),
	))
	r.MaxMultipartMemory = 512 << 20

	// Registered before the session middlewares so assets skip the session cookie and user lookup.
	r.Group("/static", staticCache()).Static("", filepath.Join("web", "static"))

	storeSess := cookie.NewStore([]byte(cfg.SecretKey))
	storeSess.Options(sessions.Options{
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 14,
	})
	r.Use(sessions.Sessions("session", storeSess))
	r.Use(middleware.LoadUser(gdb))
	r.Use(middleware.CSRF())
	r.Use(middleware.SetupGuard(st))

	docs.SwaggerInfo.Title = "CTF Platform API"
	docs.SwaggerInfo.Description = "CTFd-like REST API. Documentation is restricted to localhost."
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.BasePath = "/"
	swagger := r.Group("/swagger", middleware.LocalhostOnly())
	swagger.GET("/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/setup", app.SetupGET)
	r.POST("/setup", app.SetupPOST)
	r.GET("/", app.Index)
	r.GET("/login", app.LoginGET)
	r.POST("/login", app.LoginPOST)
	r.GET("/register", app.RegisterGET)
	r.POST("/register", app.RegisterPOST)
	r.POST("/logout", app.Logout)
	r.GET("/challenges", app.ChallengesPage)
	r.GET("/attack", middleware.Authed(), app.AttackPage)
	r.POST("/attack", middleware.Authed(), middleware.NotBanned(), app.AttackSubmit)
	r.POST("/attack/hosts", middleware.Authed(), app.AttackHostsPOST)
	r.POST("/attack/token", middleware.Authed(), app.AttackTokenPOST)
	r.GET("/scoreboard", app.ScoreboardPage)
	r.GET("/dashboard", app.DashboardPage)
	r.GET("/users", app.UsersList)
	r.GET("/users/:id", app.UserDetail)
	r.GET("/teams", app.TeamsList)
	r.GET("/teams/new", middleware.Authed(), app.TeamNewGET)
	r.POST("/teams/new", middleware.Authed(), app.TeamNewPOST)
	r.GET("/teams/join", middleware.Authed(), app.TeamJoinGET)
	r.POST("/teams/join", middleware.Authed(), app.TeamJoinPOST)
	r.GET("/teams/:id", app.TeamDetail)
	r.GET("/settings", middleware.Authed(), app.SettingsGET)
	r.POST("/settings", middleware.Authed(), app.SettingsPOST)
	r.GET("/notifications", middleware.Authed(), app.Notifications)
	r.GET("/files/:id", app.FileDownload)

	api := r.Group("/api/v1")
	{
		api.GET("/configs", app.APIConfig)
		api.GET("/challenges", app.APIChallenges)
		api.GET("/challenges/:id", app.APIChallenge)
		api.POST("/challenges/:id/attempt", middleware.Authed(), middleware.NotBanned(), app.APIAttempt)
		api.GET("/challenges/:id/solves", app.APIChallengeSolves)
		api.GET("/scoreboard", app.APIScoreboard)
		api.GET("/scoreboard/top/:count", app.APIScoreboardTop)
		api.GET("/users", app.APIUsers)
		api.GET("/users/me", middleware.Authed(), app.APIMe)
		api.GET("/users/:id", app.APIUser)
		api.GET("/teams", app.APITeams)
		api.GET("/teams/:id", app.APITeam)
		api.GET("/notifications", app.APINotifications)
		api.POST("/unlocks", middleware.Authed(), middleware.NotBanned(), app.APIUnlocks)
		api.GET("/ad/scoreboard", app.APIADScoreboard)
		api.GET("/ad/status", app.APIADStatus)
		api.GET("/ad/dashboard", app.APIADDashboard)
		api.POST("/ad/flags", app.AttackSubmit)
		api.PUT("/ad/flags", app.AttackSubmit)
	}

	admin := r.Group("/admin", middleware.Authed(), middleware.Admin())
	{
		admin.GET("", app.AdminHome)
		admin.GET("/statistics", app.AdminStatistics)
		admin.GET("/deploy/status", app.AdminDeployStatus)
		admin.GET("/challenges", app.AdminChallenges)
		admin.GET("/challenges/new", app.AdminChallengeNewGET)
		admin.POST("/challenges", app.AdminChallengeNewPOST)
		admin.POST("/challenges/upload", app.AdminChallengeUpload)
		admin.POST("/challenges/:id/redeploy", app.AdminChallengeRedeploy)
		admin.POST("/challenges/:id/stop", app.AdminChallengeStop)
		admin.GET("/challenges/:id", app.AdminChallengeEditGET)
		admin.POST("/challenges/:id", app.AdminChallengeEditPOST)
		admin.POST("/challenges/:id/delete", app.AdminChallengeDelete)
		admin.POST("/files/:id/delete", app.AdminFileDelete)
		admin.GET("/users", app.AdminUsers)
		admin.GET("/users/:id", app.AdminUserEditGET)
		admin.POST("/users/:id", app.AdminUserEditPOST)
		admin.POST("/users/:id/delete", app.AdminUserDelete)
		admin.GET("/teams", app.AdminTeams)
		admin.GET("/teams/:id", app.AdminTeamEditGET)
		admin.POST("/teams/:id", app.AdminTeamEditPOST)
		admin.POST("/teams/:id/delete", app.AdminTeamDelete)
		admin.GET("/submissions", app.AdminSubmissions)
		admin.POST("/submissions/:id/delete", app.AdminSubmissionDelete)
		admin.GET("/pages", app.AdminPages)
		admin.GET("/pages/new", app.AdminPageNewGET)
		admin.POST("/pages", app.AdminPageNewPOST)
		admin.GET("/pages/:id", app.AdminPageEditGET)
		admin.POST("/pages/:id", app.AdminPageEditPOST)
		admin.POST("/pages/:id/delete", app.AdminPageDelete)
		admin.GET("/notifications", app.AdminNotifications)
		admin.POST("/notifications", app.AdminNotificationCreate)
		admin.POST("/notifications/:id/delete", app.AdminNotificationDelete)
		admin.GET("/awards", app.AdminAwards)
		admin.POST("/awards", app.AdminAwardCreate)
		admin.POST("/awards/:id/delete", app.AdminAwardDelete)
		admin.GET("/ad", app.AdminAD)
		admin.POST("/ad/config", app.AdminADConfigPOST)
		admin.POST("/ad/services", app.AdminADServicePOST)
		admin.POST("/ad/services/:id/delete", app.AdminADServiceDelete)
		admin.POST("/ad/upload", app.AdminADUpload)
		admin.POST("/ad/services/:id/redeploy", app.AdminADRedeploy)
		admin.POST("/ad/services/:id/stop", app.AdminADStop)
		admin.POST("/ad/checkers/:id/redeploy", app.AdminADCheckerRedeploy)
		admin.POST("/ad/checkers/:id/stop", app.AdminADCheckerStop)
		admin.POST("/ad/checkers/:id/delete", app.AdminADCheckerDelete)
		admin.POST("/ad/services/:id/points", app.AdminADPointsPOST)
		admin.POST("/ad/hosts", app.AdminADHostsPOST)
		admin.GET("/config", app.AdminConfigGET)
		admin.POST("/config", app.AdminConfigPOST)
		admin.POST("/reset", app.AdminReset)
		admin.GET("/scoreboard", app.AdminScoreboard)
		admin.POST("/solves", app.AdminManualSolve)
	}

	r.GET("/:route", app.CustomPage)

	addr := ":" + cfg.Port
	srv := &http.Server{Addr: addr, Handler: r}
	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-sigCtx.Done()
		log.Printf("platform stopping: removing task containers first")
		eng.Stop()
		done := make(chan struct{})
		go func() {
			app.StopDeployedTasks()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(100 * time.Second):
			log.Printf("task container shutdown timed out")
		}
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			log.Printf("http shutdown: %v", err)
		}
	}()
	log.Printf("listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// staticCache lets browsers keep vendored and ?v= versioned assets; other static
// files are revalidated so edits show up without a hard reload.
func staticCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/static/vendor/") || c.Query("v") != "" {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "no-cache")
		}
		c.Next()
	}
}
