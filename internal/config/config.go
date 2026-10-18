package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Port        string
	SecretKey   string
	DatabaseURL string
	Driver      string
	UploadDir   string
	DataDir     string
	PublicHost  string
	TasksDir    string
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func Load() Config {
	driver := getenv("DB_DRIVER", "mysql")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		if driver == "sqlite" {
			dsn = getenv("SQLITE_PATH", "data/ctfd.db")
		} else {
			user := getenv("MYSQL_USER", "ctfd")
			pass := getenv("MYSQL_PASSWORD", "ctfd")
			host := getenv("MYSQL_HOST", "127.0.0.1")
			port := getenv("MYSQL_PORT", "3306")
			name := getenv("MYSQL_DATABASE", "ctfd")
			dsn = user + ":" + pass + "@tcp(" + host + ":" + port + ")/" + name + "?charset=utf8mb4&parseTime=True&loc=Local"
		}
	}
	dataDir := getenv("DATA_DIR", "data")
	return Config{
		Port:        getenv("PORT", "8000"),
		SecretKey:   getenv("SECRET_KEY", "change-me-please-use-a-long-random-secret"),
		DatabaseURL: dsn,
		Driver:      driver,
		UploadDir:   getenv("UPLOAD_DIR", "data/uploads"),
		DataDir:     dataDir,
		PublicHost:  getenv("PUBLIC_HOST", "127.0.0.1"),
		TasksDir:    getenv("TASKS_DIR", filepath.Join(dataDir, "tasks")),
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
