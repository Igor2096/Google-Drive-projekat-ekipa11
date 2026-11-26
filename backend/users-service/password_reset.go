package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type passwordResetRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token           string `json:"token"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func RequestPasswordResetHandler(users *UserRepository, mailer EmailSender, appURL string, validity time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		defer r.Body.Close()
		var request passwordResetRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Neispravno telo zahteva", http.StatusBadRequest)
			return
		}
		request.Email = strings.ToLower(strings.TrimSpace(request.Email))
		if request.Email == "" {
			http.Error(w, "Email je obavezan", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		user, err := users.FindByEmail(ctx, request.Email)
		if err == nil {
			token, secret, tokenErr := GeneratePasswordResetToken(user.ID)
			if tokenErr == nil {
				hash, hashErr := HashPassword(secret)
				if hashErr == nil {
					expiresAt := time.Now().UTC().Add(validity)
					if users.SetPasswordResetToken(ctx, user.ID, hash, expiresAt) == nil {
						resetURL := strings.TrimRight(appURL, "/") + "/reset-password?token=" + url.QueryEscape(token)
						body := "Zdravo " + user.FirstName + ",\n\nZa postavljanje nove lozinke otvorite sledeći link:\n" + resetURL + "\n\nLink važi do " + expiresAt.Format(time.RFC3339) + "."
						_ = mailer.Send(user.Email, "Reset lozinke", body)
					}
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Ako nalog postoji, poslali smo link za reset lozinke."})
	})
}

func ResetPasswordHandler(users *UserRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		defer r.Body.Close()
		var request resetPasswordRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			http.Error(w, "Neispravno telo zahteva", http.StatusBadRequest)
			return
		}
		if request.Password != request.ConfirmPassword {
			http.Error(w, "Lozinke se ne podudaraju", http.StatusBadRequest)
			return
		}
		if err := validateStrongPassword(request.Password); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		userID, secret, err := ParsePasswordResetToken(strings.TrimSpace(request.Token))
		if err != nil {
			http.Error(w, "Link za reset nije ispravan", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		user, err := users.FindByID(ctx, userID)
		if err != nil || user.PasswordResetExpiresAt == nil || time.Now().UTC().After(*user.PasswordResetExpiresAt) || CheckPassword(user.PasswordResetTokenHash, secret) != nil {
			http.Error(w, "Link za reset nije ispravan ili je istekao", http.StatusUnauthorized)
			return
		}
		hash, err := HashPassword(request.Password)
		if err != nil {
			http.Error(w, "Reset lozinke nije uspeo", http.StatusInternalServerError)
			return
		}
		updated, err := users.ResetPassword(ctx, userID, user.PasswordResetTokenHash, hash)
		if err != nil {
			http.Error(w, "Reset lozinke nije uspeo", http.StatusInternalServerError)
			return
		}
		if !updated {
			http.Error(w, "Link za reset nije ispravan ili je istekao", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Lozinka je uspešno resetovana"})
	})
}
