package main

import (
	"fmt"
	"strconv"
)

const defaultStorageQuotaBytes int64 = 1 << 30 // 1 GiB

type Config struct {
	Port                string
	MongoURI            string
	MongoDatabase       string
	DefaultStorageQuota int64
}

func LoadConfig() (Config, error) {
	quota, err := positiveInt64Env("DEFAULT_STORAGE_QUOTA_BYTES", defaultStorageQuotaBytes)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Port:                getEnv("PORT", "8081"),
		MongoURI:            getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:       getEnv("MONGO_DB", "google_drive"),
		DefaultStorageQuota: quota,
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
