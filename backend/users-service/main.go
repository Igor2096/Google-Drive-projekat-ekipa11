package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	config, err := LoadConfig()
	if err != nil {
		log.Fatal("Neispravna konfiguracija: ", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoURI))
	if err != nil {
		log.Fatal("Povezivanje na MongoDB nije uspelo: ", err)
	}
	defer client.Disconnect(context.Background())
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal("MongoDB nije dostupan: ", err)
	}

	users := NewUserRepository(client.Database(config.MongoDatabase).Collection("users"))
	if err := users.EnsureIndexes(ctx); err != nil {
		log.Fatal("Kreiranje MongoDB indeksa nije uspelo: ", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/register", RegistrationHandler(users, config.DefaultStorageQuota))

	log.Printf("Users servis radi na portu %s; MongoDB baza: %s", config.Port, config.MongoDatabase)
	log.Fatal(http.ListenAndServe(":"+config.Port, mux))
}
