package main

import (
	"fmt"
	"strconv"
	"time"
)

const defaultStorageQuotaBytes int64 = 1 << 30 // 1 GiB

type Config struct {
	Port                string
	MongoURI            string
	MongoDatabase       string
	DefaultStorageQuota int64
	OTPValidity         time.Duration
	TokenValidity       time.Duration
	JWTSecret           string
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
	return Config{
		Port:                getEnv("PORT", "8081"),
		MongoURI:            getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:       getEnv("MONGO_DB", "google_drive"),
		DefaultStorageQuota: quota,
		OTPValidity:         time.Duration(otpMinutes) * time.Minute,
		TokenValidity:       time.Duration(tokenMinutes) * time.Minute,
		JWTSecret:           getEnv("JWT_SECRET", "development-only-change-me"),
	}, nil
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
