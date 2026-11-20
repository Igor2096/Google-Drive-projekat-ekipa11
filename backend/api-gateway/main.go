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

	gateway_port := os.Getenv("GATEWAY_PORT")
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	filesServiceURL := os.Getenv("FILES_SERVICE_URL")
	notificationsServiceURL := os.Getenv("NOTIFICATIONS_SERVICE_URL")

	if gateway_port == "" {
		log.Fatal("GATEWAY_PORT is not defined!")
	}

	if usersServiceURL == "" || filesServiceURL == "" || notificationsServiceURL == "" {
		log.Fatal("One or more microservice URLs are missing!")
	}

	http.HandleFunc("/health", healthHandler)

	http.Handle("/api/users/", createProxy(usersServiceURL, "/api/users"))
	http.Handle("/api/files/", createProxy(filesServiceURL, "/api/files"))
	http.Handle("/api/notifications/", createProxy(notificationsServiceURL, "/api/notifications"))

	fmt.Println("API Gateway starting on port " + gateway_port + "...")

	err := http.ListenAndServe(":"+gateway_port, nil)
	if err != nil {
		log.Fatal("Server error:", err)
	}
}
