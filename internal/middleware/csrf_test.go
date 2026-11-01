package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSkipFlagCSRF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		method, path, token string
		want                bool
	}{
		{http.MethodPost, "/api/v1/ad/flags", "team-token", true},
		{http.MethodPut, "/api/v1/ad/flags", "team-token", true},
		{http.MethodPost, "/api/v1/ad/flags", "", false},
		{http.MethodPost, "/api/v1/ad/flags", "   ", false},
		{http.MethodPost, "/admin/users", "team-token", false},
		{http.MethodPost, "/attack/hosts", "team-token", false},
		{http.MethodPost, "/api/v1/challenges/1/attempt", "team-token", false},
		{http.MethodGet, "/api/v1/ad/flags", "team-token", false},
		{http.MethodPatch, "/api/v1/ad/flags", "team-token", false},
	}
	for _, tc := range cases {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(tc.method, tc.path, nil)
		if tc.token != "" {
			c.Request.Header.Set("X-Team-Token", tc.token)
		}
		if got := skipFlagCSRF(c); got != tc.want {
			t.Errorf("%s %s token %q: got %v, want %v", tc.method, tc.path, tc.token, got, tc.want)
		}
	}
}
