package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                       string
	NotificationPort           string
	DBUrl                      string
	JWTSecret                  string
	Pepper                     string
	TermiiApiKey               string
	TermiiSenderID             string
	SMTPHost                   string
	SMTPPort                   string
	SMTPUser                   string
	SMTPPass                   string
	TendarAPIKey               string
	PremblyAPIKey              string
	CBAInternalURL             string
	CBAInternalKey             string
	CBAWebhookSecret           string
	ExpoPushBaseURL            string
	ExpoAccessToken            string
	ExpoPushChannelID          string
	NotificationInternalSecret string
	ProvidusSecretKey          string
	ProvidusBaseURL            string
	ProvidusWebhookSecret      string

	LoginRateLimitIPMaxAttempts    int
	LoginRateLimitEmailMaxAttempts int
	LoginRateLimitWindowMinutes    int
	LoginRateLimitBlockMinutes     int
}

func Load() Config {
	notificationPort := getEnv("NOTIFICATION_PORT", "8081")

	return Config{
		Port:                       getEnv("PORT", "8080"),
		NotificationPort:           notificationPort,
		DBUrl:                      getEnv("DB_URL", ""),
		JWTSecret:                  getEnv("JWT_SECRET", ""),
		Pepper:                     getEnv("PEPPER", ""),
		TermiiApiKey:               getEnv("TERMII_APIKEY", ""),
		TermiiSenderID:             getEnv("TERMII_SENDERID", ""),
		SMTPHost:                   getEnv("SMTP_HOST", ""),
		SMTPPort:                   getEnv("SMTP_PORT", ""),
		SMTPUser:                   getEnv("SMTP_USER", ""),
		SMTPPass:                   getEnv("SMTP_PASS", ""),
		TendarAPIKey:               getEnv("TENDAR_APIKEY", ""),
		PremblyAPIKey:              getEnv("PREMBLY_APIKEY", ""),
		CBAInternalURL:             getEnv("CBA_INTERNAL_URL", ""),
		CBAInternalKey:             getEnv("CBA_INTERNAL_KEY", ""),
		CBAWebhookSecret:           getEnv("CBA_WEBHOOK_SECRET", ""),
		ExpoPushBaseURL:            getEnv("EXPO_PUSH_BASE_URL", "https://exp.host"),
		ExpoAccessToken:            getEnv("EXPO_ACCESS_TOKEN", ""),
		ExpoPushChannelID:          getEnv("EXPO_PUSH_CHANNEL_ID", "default"),
		NotificationInternalSecret: getEnv("NOTIFICATION_INTERNAL_SECRET", ""),
		ProvidusSecretKey:          getEnv("PROVIDUS_SECRET_KEY", ""),
		ProvidusBaseURL:            getEnv("PROVIDUS_BASE_URL", ""),
		ProvidusWebhookSecret:      getEnv("PROVIDUS_WEBHOOK_SECRET", ""),

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
