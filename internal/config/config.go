package config

import (
	"os"
)

type Config struct {
	Port           string
	DBUrl          string
	JWTSecret      string
	Pepper         string
	TermiiApiKey   string
	TermiiSenderID string
	SMTPHost       string
	SMTPPort       string
	SMTPUser       string
	SMTPPass       string
	TendarAPIKey   string
	PremblyAPIKey  string
	CBAInternalURL string
	CBAInternalKey string
}

func Load() Config {
	return Config{
		Port:           getEnv("PORT", "8080"),
		DBUrl:          getEnv("DB_URL", ""),
		JWTSecret:      getEnv("JWT_SECRET", ""),
		Pepper:         getEnv("PEPPER", ""),
		TermiiApiKey:   getEnv("TERMII_APIKEY", ""),
		TermiiSenderID: getEnv("TERMII_SENDERID", ""),
		SMTPHost:       getEnv("SMTP_HOST", ""),
		SMTPPort:       getEnv("SMTP_PORT", ""),
		SMTPUser:       getEnv("SMTP_USER", ""),
		SMTPPass:       getEnv("SMTP_PASS", ""),
		TendarAPIKey:   getEnv("TENDAR_APIKEY", ""),
		PremblyAPIKey:  getEnv("PREMBLY_APIKEY", ""),
		CBAInternalURL: getEnv("CBA_INTERNAL_URL", ""),
		CBAInternalKey: getEnv("CBA_INTERNAL_KEY", ""),
	}
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}
	return value
}
