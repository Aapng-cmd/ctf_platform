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
	Port          string
	SecretKey     string
	DatabaseURL   string
	Driver        string
	UploadDir     string
	DataDir       string
	PublicHost    string
	TasksDir      string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
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
		Port:          getenv("PORT", "8000"),
		SecretKey:     getenv("SECRET_KEY", "change-me-please-use-a-long-random-secret"),
		DatabaseURL:   dsn,
		Driver:        driver,
		UploadDir:     getenv("UPLOAD_DIR", "data/uploads"),
		DataDir:       dataDir,
		PublicHost:    getenv("PUBLIC_HOST", "127.0.0.1"),
		TasksDir:      getenv("TASKS_DIR", filepath.Join(dataDir, "tasks")),
		RedisAddr:     getenv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB:       EnvInt("REDIS_DB", 0),
	}
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
