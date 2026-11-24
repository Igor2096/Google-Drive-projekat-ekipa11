package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func LoginHandler(users *UserRepository, otpValidity time.Duration, mailer EmailSender) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		defer r.Body.Close()

		var request loginRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			http.Error(w, "Neispravno telo zahteva", http.StatusBadRequest)
			return
		}
		request.Email = strings.ToLower(strings.TrimSpace(request.Email))
		if request.Email == "" || request.Password == "" {
			http.Error(w, "Email i lozinka su obavezni", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		user, err := users.FindByEmail(ctx, request.Email)
		if err != nil || CheckPassword(user.PasswordHash, request.Password) != nil {
			if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
				log.Printf("Neuspešna provera prijave za %q: %v", request.Email, err)
			}
			http.Error(w, "Neispravan email ili lozinka", http.StatusUnauthorized)
			return
		}

		otp, err := GenerateOTP()
		if err != nil {
			http.Error(w, "Generisanje OTP koda nije uspelo", http.StatusInternalServerError)
			return
		}
		otpHash, err := HashPassword(otp)
		if err != nil {
			http.Error(w, "Obrada OTP koda nije uspela", http.StatusInternalServerError)
			return
		}
		expiresAt := time.Now().UTC().Add(otpValidity)
		if err := users.SetLoginOTP(ctx, user.ID, otpHash, expiresAt); err != nil {
			http.Error(w, "Prijava nije uspela", http.StatusInternalServerError)
			return
		}

		body := "Zdravo " + user.FirstName + ",\n\nVaš jednokratni kod za prijavu je: " + otp + "\n\nKod važi do " + expiresAt.Format(time.RFC3339) + "."
		if err := mailer.Send(user.Email, "Kod za prijavu", body); err != nil {
			log.Printf("Slanje OTP email poruke korisniku %q nije uspelo: %v", user.Email, err)
			http.Error(w, "Slanje OTP koda nije uspelo", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "OTP kod je poslat na email adresu"})
	})
}
