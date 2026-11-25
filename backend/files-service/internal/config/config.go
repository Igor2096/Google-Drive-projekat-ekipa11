// Package config cita konfiguraciju iz environment promenljivih (zahtev 2.4).
// Nista od ovoga nije zakucano u kod - samo podrazumevane vrednosti za lokalni rad.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           string        // port na kom servis slusa
	MongoURI       string        // npr. mongodb://mongodb:27017
	MongoDB        string        // ime baze za files servis
	AllowedOrigins []string      // CORS (Angular dev server dok nema gateway-a)
	DBTimeout      time.Duration // timeout za pojedinacnu operaciju nad bazom
	StoragePath    string        // gde se cuva sadrzaj datoteka (T-23)
	MaxUploadSize  int64         // najveca dozvoljena velicina jednog zahteva za otpremanje
}

func Load() Config {
	return Config{
		Port:           getEnv("FILES_SERVICE_PORT", "8082"),
		MongoURI:       getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:        getEnv("FILES_DB_NAME", "files_db"),
		AllowedOrigins: strings.Split(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:4200"), ","),
		DBTimeout:      getDuration("DB_TIMEOUT", 5*time.Second),
		StoragePath:    getEnv("STORAGE_PATH", "./data/files"),
		MaxUploadSize:  getBytes("MAX_UPLOAD_SIZE", 50<<20), // 50 MB
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getBytes(key string, fallback int64) int64 {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
