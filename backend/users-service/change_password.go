package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

type changePasswordRequest struct {
	Email       string `json:"email"`
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func ChangePasswordHandler(users *UserRepository, minimumAge time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		defer r.Body.Close()
		var request changePasswordRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			http.Error(w, "Neispravno telo zahteva", http.StatusBadRequest)
			return
		}
		request.Email = strings.ToLower(strings.TrimSpace(request.Email))
		if request.Email == "" || request.OldPassword == "" || request.NewPassword == "" {
			http.Error(w, "Email, stara i nova lozinka su obavezni", http.StatusBadRequest)
			return
		}
		if err := validateStrongPassword(request.NewPassword); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		user, err := users.FindByEmail(ctx, request.Email)
		if err != nil || CheckPassword(user.PasswordHash, request.OldPassword) != nil {
			if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
				http.Error(w, "Promena lozinke nije uspela", http.StatusInternalServerError)
				return
			}
			http.Error(w, "Email ili stara lozinka nisu ispravni", http.StatusUnauthorized)
			return
		}
		if !PasswordCanBeChanged(user.PasswordChangedAt, time.Now().UTC(), minimumAge) {
			http.Error(w, "Lozinka može biti promenjena tek nakon 24 časa", http.StatusConflict)
			return
		}
		if CheckPassword(user.PasswordHash, request.NewPassword) == nil {
			http.Error(w, "Nova lozinka mora biti različita od stare", http.StatusBadRequest)
			return
		}
		hash, err := HashPassword(request.NewPassword)
		if err != nil {
			http.Error(w, "Promena lozinke nije uspela", http.StatusInternalServerError)
			return
		}
		if err := users.UpdatePassword(ctx, user.ID, hash); err != nil {
			http.Error(w, "Promena lozinke nije uspela", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Lozinka je uspešno promenjena"})
	})
}
