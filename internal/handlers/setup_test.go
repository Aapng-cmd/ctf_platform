package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/aapng-cmd/ctf_platform/internal/config"
	"github.com/aapng-cmd/ctf_platform/internal/db"
	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/store"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSetupReturns404AfterAdminExists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gdb, err := gorm.Open(sqlite.Open("file:setup404?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(gdb); err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&models.User{Name: "root", Email: "root@example.com", Password: "x", Type: models.UserTypeAdmin}).Error; err != nil {
		t.Fatal(err)
	}
	root := repoRoot(t)
	t.Chdir(root)
	app, err := New(gdb, store.New(gdb), config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, "/setup", nil)
		if method == http.MethodGet {
			app.SetupGET(c)
		} else {
			app.SetupPOST(c)
		}
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s /setup = %d, want 404", method, w.Code)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "web", "templates", "setup.html")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root not found")
		}
		dir = parent
	}
}
