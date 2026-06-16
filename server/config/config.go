package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config holds runtime configuration loaded from environment / .env.
type Config struct {
	Port               string
	JWTSecret          string
	SuperAdminUsername string
	SuperAdminPassword string
	DBPath             string
	UploadDir          string
	EncryptionKey      string // used to encrypt secret settings (e.g. AI api key)
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Load reads configuration from the environment, loading a .env file first if present.
func Load() *Config {
	_ = godotenv.Load()

	c := &Config{
		Port:               getenv("PORT", "8080"),
		JWTSecret:          getenv("JWT_SECRET", "murmur-dev-secret-change-me"),
		SuperAdminUsername: getenv("SUPER_ADMIN_USERNAME", "admin"),
		SuperAdminPassword: getenv("SUPER_ADMIN_PASSWORD", "admin12345"),
		DBPath:             getenv("DB_PATH", "./data/murmur.db"),
		UploadDir:          getenv("UPLOAD_DIR", "./uploads"),
		EncryptionKey:      os.Getenv("SETTINGS_ENC_KEY"),
	}
	// Fall back to deriving the encryption key from the JWT secret so secrets
	// are still encrypted at rest without extra configuration.
	if c.EncryptionKey == "" {
		c.EncryptionKey = "enc:" + c.JWTSecret
	}
	return c
}
