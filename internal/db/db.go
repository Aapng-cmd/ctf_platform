package db

import (
	"os"
	"path/filepath"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/config"
	"github.com/aapng-cmd/ctf_platform/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	var (
		dial gorm.Dialector
		err  error
	)
	switch cfg.Driver {
	case "sqlite":
		if err = os.MkdirAll(filepath.Dir(cfg.DatabaseURL), 0o755); err != nil && !os.IsExist(err) {
			if filepath.Dir(cfg.DatabaseURL) != "." {
				return nil, err
			}
		}
		dial = sqlite.Open(cfg.DatabaseURL)
	default:
		dial = postgres.Open(cfg.DatabaseURL)
	}

	var gdb *gorm.DB
	for i := 0; i < 30; i++ {
		gdb, err = gorm.Open(dial, &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(8)
	sqlDB.SetMaxOpenConns(32)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return gdb, nil
}

func Migrate(gdb *gorm.DB) error {
	return gdb.AutoMigrate(
		&models.User{},
		&models.Team{},
		&models.Challenge{},
		&models.Flag{},
		&models.Hint{},
		&models.File{},
		&models.Tag{},
		&models.Submission{},
		&models.Solve{},
		&models.Unlock{},
		&models.Award{},
		&models.Page{},
		&models.Notification{},
		&models.ConfigKV{},
		&models.Tracking{},
		&models.ADChecker{},
		&models.ADService{},
		&models.ADInstance{},
		&models.ADRound{},
		&models.ADSteal{},
		&models.ADCheck{},
	)
}
