package config

import (
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Port           string
	SecretKey      string
	DatabaseURL    string
	Driver         string
	UploadDir      string
	DataDir        string
	PublicHost     string
	TasksDir       string
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	CookieSecure   bool
	PublicURL      string
	TrustedProxies []string
	// RunEngine is true on the single process that advances Attack-Defense rounds.
	// Extra web copies set AD_ENGINE=false and must not receive the Docker socket.
	RunEngine bool
	// DBMaxOpen is this process's database pool, clamped to 4..128.
	DBMaxOpen int
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func Load() Config {
	driver := getenv("DB_DRIVER", "postgres")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		if driver == "sqlite" {
			dsn = getenv("SQLITE_PATH", "data/ctfd.db")
		} else {
			user := getenv("POSTGRES_USER", "ctfd")
			pass := getenv("POSTGRES_PASSWORD", "ctfd")
			host := getenv("POSTGRES_HOST", "127.0.0.1")
			port := getenv("POSTGRES_PORT", "5432")
			name := getenv("POSTGRES_DB", "ctfd")
			u := url.URL{
				Scheme:   "postgres",
				User:     url.UserPassword(user, pass),
				Host:     net.JoinHostPort(host, port),
				Path:     "/" + name,
				RawQuery: "sslmode=disable",
			}
			dsn = u.String()
		}
	}
	dataDir := getenv("DATA_DIR", "data")
	return Config{
		Port:           getenv("PORT", "8000"),
		SecretKey:      getenv("SECRET_KEY", "change-me-please-use-a-long-random-secret"),
		DatabaseURL:    dsn,
		Driver:         driver,
		UploadDir:      getenv("UPLOAD_DIR", "data/uploads"),
		DataDir:        dataDir,
		PublicHost:     getenv("PUBLIC_HOST", "127.0.0.1"),
		TasksDir:       getenv("TASKS_DIR", filepath.Join(dataDir, "tasks")),
		RedisAddr:      getenv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:  os.Getenv("REDIS_PASSWORD"),
		RedisDB:        EnvInt("REDIS_DB", 0),
		CookieSecure:   IsTruthy(os.Getenv("COOKIE_SECURE")),
		PublicURL:      strings.TrimRight(os.Getenv("PUBLIC_URL"), "/"),
		TrustedProxies: splitList(os.Getenv("TRUSTED_PROXIES")),
		RunEngine:      envDefaultTrue("AD_ENGINE"),
		DBMaxOpen:      clamp(EnvInt("DB_MAX_OPEN", 32), 4, 128),
	}
}

func envDefaultTrue(key string) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return true
	}
	return IsTruthy(v)
}

func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

func splitList(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func EnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func IsTruthy(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
