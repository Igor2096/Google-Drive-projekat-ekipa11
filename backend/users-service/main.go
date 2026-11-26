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

	database := client.Database(config.MongoDatabase)

	users := NewUserRepository(database.Collection("users"))
	if err := users.EnsureIndexes(ctx); err != nil {
		log.Fatal("Kreiranje MongoDB indeksa nije uspelo: ", err)
	}

	quotaRequests := NewQuotaRequestRepository(
		database.Collection("quota_requests"),
	)

	if err := quotaRequests.EnsureIndexes(ctx); err != nil {
		log.Fatal("Kreiranje indeksa za zahteve kvote nije uspelo: ", err)
	}

	mux := http.NewServeMux()
	mailer := NewSMTPEmailSender(config.Mail)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.Handle("/register", RegistrationHandler(users, config.DefaultStorageQuota, mailer, config.Mail.AppURL, config.EmailValidity))
	mux.Handle("/confirm-email", ConfirmEmailHandler(users))
	mux.Handle("/login", LoginHandler(users, config.OTPValidity, mailer))
	mux.Handle("/verify-otp", VerifyOTPHandler(users, config.JWTSecret, config.TokenValidity))

	mux.Handle("/users/", UserQuotaHandler(users))

	mux.Handle(
		"/quota-requests",
		CreateQuotaRequestHandler(users, quotaRequests),
	)

	// Administratorske rute.
	mux.Handle("/admin/users", AdminUsersHandler(users))

	adminQuotaRequestsHandler := AdminQuotaRequestsHandler(users, quotaRequests)

	mux.Handle(
		"/admin/quota-requests",
		adminQuotaRequestsHandler,
	)

	mux.Handle(
		"/admin/quota-requests/",
		adminQuotaRequestsHandler,
	)

	log.Printf("Users servis radi na portu %s; MongoDB baza: %s", config.Port, config.MongoDatabase)

	log.Fatal(http.ListenAndServe(":"+config.Port, mux))
}
