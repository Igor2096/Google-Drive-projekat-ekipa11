package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Notification - jedan red iz tabele notifications_by_user.
type Notification struct {
	UserID         string    `json:"user_id"`
	NotificationID string    `json:"notification_id"`
	Message        string    `json:"message"`
	CreatedAt      time.Time `json:"created_at"`
	IsRead         bool      `json:"is_read"`
}

// notificationStore - ono sto handleru treba od baze; u testovima se menja fake-om.
type notificationStore interface {
	ForUser(ctx context.Context, userID string) ([]Notification, error)
}

// userIDHeader postavlja API gateway, iz proverenog JWT tokena.
// Servis NIKAD ne uzima ID korisnika iz URL-a ili tela zahteva: tada bi bilo
// ko mogao da procita tudje notifikacije samo promenom parametra (2.17).
const userIDHeader = "X-User-ID"

// GET /notifications - notifikacije prijavljenog korisnika, najnovije prve.
func notificationsHandler(store notificationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeError(w, http.StatusMethodNotAllowed, "metod nije dozvoljen")
			return
		}

		userID := r.Header.Get(userIDHeader)
		if !validUserID(userID) {
			writeError(w, http.StatusUnauthorized, "korisnik nije autentifikovan")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		notifications, err := store.ForUser(ctx, userID)
		if err != nil {
			log.Printf("citanje notifikacija nije uspelo: %v", err)
			writeError(w, http.StatusInternalServerError, "notifikacije trenutno nisu dostupne")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(notifications)
	}
}

// validUserID - ID korisnika je Mongo ObjectID: tacno 24 heksadecimalna znaka.
// Provera je i odbrana od ubacivanja bilo cega drugog u upit (2.18).
func validUserID(id string) bool {
	if len(id) != 24 {
		return false
	}
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
