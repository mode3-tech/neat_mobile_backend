package config

import (
	"os"
)

type Config struct {
	Port            string
	DBUrl           string
	JWTSecret       string
	Pepper          string
	SMSLiveApiKey   string
	SMSLiveSenderID string
	SMTPHost        string
	SMTPPort        string
	SMTPUser        string
	SMTPPass        string
}

func Load() Config {
	return Config{
		Port:            getEnv("PORT", "8080"),
		DBUrl:           getEnv("DB_URL", ""),
		JWTSecret:       getEnv("JWT_SECRET", ""),
		Pepper:          getEnv("PEPPER", ""),
		SMSLiveApiKey:   getEnv("SMSLIVE_APIKEY", ""),
		SMSLiveSenderID: getEnv("SMSLIVE_SENDERID", ""),
		SMTPHost:        getEnv("SMTP_HOST", ""),
		SMTPPort:        getEnv("SMTP_PORT", ""),
		SMTPUser:        getEnv("SMTP_USER", ""),
		SMTPPass:        getEnv("SMTP_PASS", ""),
	}
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}
	return value
}
