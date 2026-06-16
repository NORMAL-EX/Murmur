// Package db wires up GORM + SQLite, runs migrations and seeds initial data.
package db

import (
	"os"
	"path/filepath"

	"murmur/auth"
	"murmur/config"
	"murmur/models"
	"murmur/settings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Init(cfg *config.Config) (*gorm.DB, error) {
	if dir := filepath.Dir(cfg.DBPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	dsn := cfg.DBPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	if err := gdb.AutoMigrate(
		&models.User{},
		&models.Setting{},
		&models.Channel{},
		&models.Message{},
		&models.DirectMessage{},
		&models.Mention{},
		&models.Reaction{},
		&models.AuditLog{},
	); err != nil {
		return nil, err
	}
	return gdb, nil
}

// Seed creates the super admin, the bot account and the default channel if missing.
func Seed(gdb *gorm.DB, cfg *config.Config, st *settings.Service) error {
	// Super admin (idempotent): create if absent, otherwise leave untouched.
	var sa models.User
	err := gdb.Where("username = ?", cfg.SuperAdminUsername).First(&sa).Error
	if err == gorm.ErrRecordNotFound {
		hash, herr := auth.HashPassword(cfg.SuperAdminPassword)
		if herr != nil {
			return herr
		}
		if cerr := gdb.Create(&models.User{
			Username:        cfg.SuperAdminUsername,
			PasswordHash:    hash,
			Nickname:        cfg.SuperAdminUsername,
			Role:            models.RoleSuperAdmin,
			Status:          models.StatusActive,
			RateLimitPerMin: models.RateInherit,
		}).Error; cerr != nil {
			return cerr
		}
	} else if err == nil && sa.Role != models.RoleSuperAdmin {
		// Ensure the configured account keeps super admin rights.
		gdb.Model(&sa).Update("role", models.RoleSuperAdmin)
	}

	// Bot account.
	var botCount int64
	gdb.Model(&models.User{}).Where("role = ?", models.RoleBot).Count(&botCount)
	if botCount == 0 {
		if err := gdb.Create(&models.User{
			Username:        "bot",
			Nickname:        st.Get(settings.BotName),
			AvatarURL:       st.Get(settings.BotAvatar),
			Role:            models.RoleBot,
			Status:          models.StatusActive,
			RateLimitPerMin: models.RateUnlimited,
		}).Error; err != nil {
			return err
		}
	}

	// Default channel.
	var chCount int64
	gdb.Model(&models.Channel{}).Count(&chCount)
	if chCount == 0 {
		if err := gdb.Create(&models.Channel{
			Name:        "大厅",
			Slug:        "lobby",
			Description: "公共聊天大厅,欢迎来到 Murmur",
			Pinned:      true,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// BotUser returns the bot account.
func BotUser(gdb *gorm.DB) (*models.User, error) {
	var u models.User
	if err := gdb.Where("role = ?", models.RoleBot).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}
