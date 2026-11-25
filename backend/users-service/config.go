package main

import (
	"fmt"
	"strconv"
	"time"
)

const defaultStorageQuotaBytes int64 = 1 << 30

type Config struct {
	Port                     string
	MongoURI                 string
	MongoDatabase            string
	DefaultStorageQuota      int64
	OTPValidity              time.Duration
	TokenValidity            time.Duration
	EmailValidity            time.Duration
	PasswordChangeMinimumAge time.Duration
	PasswordResetValidity    time.Duration
	PasswordMaxAge           time.Duration
	JWTSecret                string
	Mail                     MailConfig
}

func LoadConfig() (Config, error) {
	quota, err := positiveInt64Env("DEFAULT_STORAGE_QUOTA_BYTES", defaultStorageQuotaBytes)
	if err != nil {
		return Config{}, err
	}
	tokenMinutes, err := positiveInt64Env("TOKEN_TTL_MINUTES", 60)
	if err != nil {
		return Config{}, err
	}
	otpMinutes, err := positiveInt64Env("OTP_TTL_MINUTES", 10)
	if err != nil {
		return Config{}, err
	}
	emailHours, err := positiveInt64Env("EMAIL_VERIFICATION_TTL_HOURS", 24)
	if err != nil {
		return Config{}, err
	}
	passwordChangeHours, err := positiveInt64Env("PASSWORD_CHANGE_MIN_AGE_HOURS", 24)
	if err != nil {
		return Config{}, err
	}
	passwordResetMinutes, err := positiveInt64Env("PASSWORD_RESET_TTL_MINUTES", 30)
	if err != nil {
		return Config{}, err
	}
	passwordMaxAge, err := passwordMaxAgeFromEnv()
	if err != nil {
		return Config{}, err
	}
	smtpPort, err := positiveInt64Env("SMTP_PORT", 1025)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Port:                     getEnv("PORT", "8081"),
		MongoURI:                 getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:            getEnv("MONGO_DB", "google_drive"),
		DefaultStorageQuota:      quota,
		OTPValidity:              time.Duration(otpMinutes) * time.Minute,
		TokenValidity:            time.Duration(tokenMinutes) * time.Minute,
		EmailValidity:            time.Duration(emailHours) * time.Hour,
		PasswordChangeMinimumAge: time.Duration(passwordChangeHours) * time.Hour,
		PasswordResetValidity:    time.Duration(passwordResetMinutes) * time.Minute,
		PasswordMaxAge:           passwordMaxAge,
		JWTSecret:                getEnv("JWT_SECRET", "development-only-change-me"),
		Mail: MailConfig{
			Host:     getEnv("SMTP_HOST", "localhost"),
			Port:     int(smtpPort),
			Username: getEnv("SMTP_USERNAME", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", "noreply@google-drive.local"),
			AppURL:   getEnv("APP_BASE_URL", "http://localhost:8081"),
		},
	}, nil
}

func passwordMaxAgeFromEnv() (time.Duration, error) {
	if getEnv("PASSWORD_MAX_AGE_MINUTES", "") != "" {
		minutes, err := positiveInt64Env("PASSWORD_MAX_AGE_MINUTES", 1)
		if err != nil {
			return 0, err
		}
		return time.Duration(minutes) * time.Minute, nil
	}
	days, err := positiveInt64Env("PASSWORD_MAX_AGE_DAYS", 60)
	if err != nil {
		return 0, err
	}
	return time.Duration(days) * 24 * time.Hour, nil
}

func positiveInt64Env(key string, fallback int64) (int64, error) {
	value := getEnv(key, "")
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s mora biti pozitivan ceo broj", key)
	}
	return parsed, nil
}
