package config

import (
	"os"
	"strconv"
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

	LoginRateLimitIPMaxAttempts    int
	LoginRateLimitEmailMaxAttempts int
	LoginRateLimitWindowMinutes    int
	LoginRateLimitBlockMinutes     int
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

		LoginRateLimitIPMaxAttempts:    getEnvInt("LOGIN_RATE_LIMIT_IP_MAX_ATTEMPTS", 20),
		LoginRateLimitEmailMaxAttempts: getEnvInt("LOGIN_RATE_LIMIT_EMAIL_MAX_ATTEMPTS", 5),
		LoginRateLimitWindowMinutes:    getEnvInt("LOGIN_RATE_LIMIT_WINDOW_MINUTES", 15),
		LoginRateLimitBlockMinutes:     getEnvInt("LOGIN_RATE_LIMIT_BLOCK_MINUTES", 15),
	}
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}
