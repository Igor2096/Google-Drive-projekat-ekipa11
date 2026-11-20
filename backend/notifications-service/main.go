package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gocql/gocql"
	"github.com/joho/godotenv"
)

type Notification struct {
	UserID         gocql.UUID `json:"user_id"`
	NotificationID gocql.UUID `json:"notification_id"`
	Message        string     `json:"message"`
	CreatedAt      time.Time  `json:"created_at"`
	IsRead         bool       `json:"is_read"`
}

func connectToCassandra() *gocql.Session {
	host := os.Getenv("CASSANDRA_HOST")
	if host == "" {
		log.Fatal("CASSANDRA_HOST is not defined!")
	}

	portString := os.Getenv("CASSANDRA_PORT")
	port, err := strconv.Atoi(portString)
	if err != nil {
		log.Fatal("Invalid CASSANDRA_PORT:", err)
	}

	cluster := gocql.NewCluster(host)
	cluster.Port = port
	cluster.Keyspace = "notifications_db"
	cluster.Consistency = gocql.One

	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatal("Cannot connect to Cassandra:", err)
	}

	fmt.Println("Successfully connected to Cassandra!")

	return session
}

func getNotifications(session *gocql.Session, userID gocql.UUID) ([]Notification, error) {
	var notifications []Notification

	iter := session.Query(`
        SELECT user_id, notification_id, message, created_at, is_read
        FROM notifications
        WHERE user_id = ?
    `, userID).Iter()

	var notification Notification

	for iter.Scan(
		&notification.UserID,
		&notification.NotificationID,
		&notification.Message,
		&notification.CreatedAt,
		&notification.IsRead,
	) {
		notifications = append(notifications, notification)
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}

	return notifications, nil
}

func notificationsHandler(session *gocql.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Provera HTTP metode
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Uzimanje user ID-a iz URL-a
		userIDString := r.URL.Query().Get("user_id")

		// Provera da li je user ID prosleđen
		if userIDString == "" {
			http.Error(w, "User ID is required", http.StatusBadRequest)
			return
		}

		// Pretvaranje stringa u Cassandra UUID
		userID, err := gocql.ParseUUID(userIDString)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		// Dobavljanje obaveštenja iz baze
		notifications, err := getNotifications(session, userID)
		if err != nil {
			http.Error(w, "Failed to retrieve notifications", http.StatusInternalServerError)
			return
		}

		// Vraćanje obaveštenja u JSON formatu
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(notifications)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Notifications Service is running!")
}

func main() {
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("No .env file found; using environment variables")
	}
	session := connectToCassandra()
	defer session.Close()

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/notifications", notificationsHandler(session))

	fmt.Println("Notifications Service starting on port 8083...")

	port := os.Getenv("NOTIFICATIONS_SERVICE_PORT")

	if port == "" {
		log.Fatal("NOTIFICATIONS_SERVICE_PORT is not defined!")
	}

	fmt.Println("Notifications Service starting on port " + port + "...")

	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("Server error:", err)
	}
}
