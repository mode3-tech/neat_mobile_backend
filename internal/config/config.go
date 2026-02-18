package config

import (
	"os"
)

type Config struct {
	Port      string
	DBUrl     string
	JWTSecret string
}

func Load() Config {
	return Config{
		Port:      getEnv("PORT", "8080"),
		DBUrl:     getEnv("DB_URL", ""),
		JWTSecret: getEnv("JWT_SECRET", ""),
	}
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}
	return value
}
