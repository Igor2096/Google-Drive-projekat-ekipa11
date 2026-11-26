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

type verifyOTPRequest struct {
	Email string `json:"email"`
	OTP   string `json:"otp"`
}

func VerifyOTPHandler(users *UserRepository, jwtSecret string, tokenValidity time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		defer r.Body.Close()

		var request verifyOTPRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			http.Error(w, "Neispravno telo zahteva", http.StatusBadRequest)
			return
		}
		request.Email = strings.ToLower(strings.TrimSpace(request.Email))
		request.OTP = strings.TrimSpace(request.OTP)
		if request.Email == "" || len(request.OTP) != otpLength {
			http.Error(w, "Email i OTP kod su obavezni", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		user, err := users.FindByEmail(ctx, request.Email)
		if err != nil || user.OTPExpiresAt == nil || time.Now().UTC().After(*user.OTPExpiresAt) {
			if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
				http.Error(w, "Verifikacija OTP koda nije uspela", http.StatusInternalServerError)
				return
			}
			http.Error(w, "OTP kod nije ispravan ili je istekao", http.StatusUnauthorized)
			return
		}
		if CheckPassword(user.OTPHash, request.OTP) != nil {
			attempts, err := users.RegisterFailedOTPAttempt(ctx, user.ID)
			if err == nil && otpAttemptsExhausted(attempts) {
				_ = users.ClearLoginOTP(ctx, user.ID)
				http.Error(w, "Previše pogrešnih pokušaja. Prijavite se ponovo.", http.StatusUnauthorized)
				return
			}
			http.Error(w, "OTP kod nije ispravan ili je istekao", http.StatusUnauthorized)
			return
		}

		consumed, err := users.ConsumeLoginOTP(ctx, user.ID, user.OTPHash)
		if err != nil {
			http.Error(w, "Verifikacija OTP koda nije uspela", http.StatusInternalServerError)
			return
		}
		if !consumed {
			http.Error(w, "OTP kod nije ispravan ili je istekao", http.StatusUnauthorized)
			return
		}

		accessToken, expiresAt, err := IssueAccessToken(user, jwtSecret, tokenValidity)
		if err != nil {
			http.Error(w, "Izdavanje tokena nije uspelo", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": accessToken, "token_type": "Bearer", "expires_at": expiresAt})
	})
}
