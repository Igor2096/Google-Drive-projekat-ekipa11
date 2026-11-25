package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func ConfirmEmailHandler(users *UserRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		userID, secret, err := ParseEmailVerificationToken(token)
		if err != nil {
			http.Error(w, "Link za potvrdu nije ispravan", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		user, err := users.FindByID(ctx, userID)
		if err != nil || user.EmailVerificationExpiresAt == nil || time.Now().UTC().After(*user.EmailVerificationExpiresAt) || CheckPassword(user.EmailVerificationTokenHash, secret) != nil {
			http.Error(w, "Link za potvrdu nije ispravan ili je istekao", http.StatusUnauthorized)
			return
		}
		confirmed, err := users.ConfirmEmail(ctx, userID, user.EmailVerificationTokenHash)
		if err != nil {
			http.Error(w, "Potvrda email adrese nije uspela", http.StatusInternalServerError)
			return
		}
		if !confirmed {
			http.Error(w, "Link za potvrdu nije ispravan ili je istekao", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Email adresa je uspešno potvrđena"})
	})
}
