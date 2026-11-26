package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "API Gateway is running!")
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if origin == "http://localhost:4200" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID, X-User-Role")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func createProxy(serviceURL string, prefix string) http.Handler {
	target, err := url.Parse(serviceURL)
	if err != nil {
		log.Fatal("Invalid service URL:", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	return http.StripPrefix(prefix, proxy)
}

func main() {
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("No .env file found; using environment variables")
	}

	gatewayPort := os.Getenv("GATEWAY_PORT")
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	filesServiceURL := os.Getenv("FILES_SERVICE_URL")
	notificationsServiceURL := os.Getenv("NOTIFICATIONS_SERVICE_URL")
	jwtSecret := os.Getenv("JWT_SECRET")

	if gatewayPort == "" {
		log.Fatal("GATEWAY_PORT is not defined!")
	}

	if usersServiceURL == "" || filesServiceURL == "" || notificationsServiceURL == "" {
		log.Fatal("One or more microservice URLs are missing!")
	}

	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is not defined!")
	}

	http.HandleFunc("/health", healthHandler)

	usersProxy := createProxy(usersServiceURL, "/api/users")

	// Javne rute: registracija, login, OTP i slično.
	http.Handle("/api/users/", usersProxy)

	// Korisničke kvote: potreban je validan JWT.
	userQuotaProxy := authMiddleware(jwtSecret, usersProxy)
	http.Handle("/api/users/users/", userQuotaProxy)

	// Zahtevi za kvotu: potreban je validan JWT.
	quotaRequestsProxy := authMiddleware(jwtSecret, usersProxy)
	http.Handle("/api/users/quota-requests", quotaRequestsProxy)
	http.Handle("/api/users/quota-requests/", quotaRequestsProxy)

	// Administratorske rute.
	adminUsersProxy := adminAuthMiddleware(jwtSecret, usersProxy)
	http.Handle("/api/users/admin/users", adminUsersProxy)
	http.Handle("/api/users/admin/users/", adminUsersProxy)
	http.Handle("/api/users/admin/quota-requests", adminUsersProxy)
	http.Handle("/api/users/admin/quota-requests/", adminUsersProxy)

	// Files servis.
	filesProxy := createProxy(filesServiceURL, "/api/files")
	http.Handle("/api/files/", authMiddleware(jwtSecret, filesProxy))

	// Notifications servis.
	notificationsProxy := createProxy(notificationsServiceURL, "/api/notifications")
	http.Handle("/api/notifications/", authMiddleware(jwtSecret, notificationsProxy))

	fmt.Println("API Gateway starting on port " + gatewayPort + "...")

	handler := corsMiddleware(http.DefaultServeMux)

	if err := http.ListenAndServe(":"+gatewayPort, handler); err != nil {
		log.Fatal("Server error:", err)
	}
}
