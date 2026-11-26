package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type magicLinkRequest struct {
	Email string `json:"email"`
}

type magicLinkVerifyRequest struct {
	Token string `json:"token"`
}

func RequestMagicLinkHandler(users *UserRepository, mailer EmailSender, appURL string, validity time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		defer r.Body.Close()

		var request magicLinkRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
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

		if user, err := users.FindByEmail(ctx, request.Email); err == nil && user.Verified {
			if err := sendMagicLink(ctx, users, mailer, user, appURL, validity); err != nil {
				log.Printf("slanje magicnog linka nije uspelo: %v", err)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "Ako nalog postoji, poslali smo link za prijavu na vaš email.",
		})
	})
}

func sendMagicLink(ctx context.Context, users *UserRepository, mailer EmailSender, user *User, appURL string, validity time.Duration) error {
	token, secret, err := GeneratePasswordResetToken(user.ID)
	if err != nil {
		return err
	}
	hash, err := HashPassword(secret)
	if err != nil {
		return err
	}
	expiresAt := time.Now().UTC().Add(validity)
	if err := users.SetMagicLinkToken(ctx, user.ID, hash, expiresAt); err != nil {
		return err
	}
	link := strings.TrimRight(appURL, "/") + "/magic-link?token=" + url.QueryEscape(token)
	body := "Zdravo " + user.FirstName + ",\n\nZa prijavu bez lozinke otvorite sledeći link:\n" + link +
		"\n\nLink se može iskoristiti samo jednom i važi do " + expiresAt.Format(time.RFC3339) + "." +
		"\nAko niste tražili prijavu, slobodno ignorišite ovaj email."
	return mailer.Send(user.Email, "Prijava na Disk", body)
}

func VerifyMagicLinkHandler(users *UserRepository, jwtSecret string, tokenValidity, passwordMaxAge time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		defer r.Body.Close()

		var request magicLinkVerifyRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			http.Error(w, "Neispravno telo zahteva", http.StatusBadRequest)
			return
		}

		userID, secret, err := ParsePasswordResetToken(strings.TrimSpace(request.Token))
		if err != nil {
			http.Error(w, "Link za prijavu nije ispravan", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		user, err := users.FindByID(ctx, userID)
		if err != nil || user.MagicLinkExpiresAt == nil || time.Now().UTC().After(*user.MagicLinkExpiresAt) ||
			CheckPassword(user.MagicLinkTokenHash, secret) != nil {
			http.Error(w, "Link za prijavu nije ispravan ili je istekao", http.StatusUnauthorized)
			return
		}
		if !user.Verified {
			http.Error(w, "Nalog nije potvrđen", http.StatusForbidden)
			return
		}
		if PasswordExpired(user.PasswordChangedAt, user.CreatedAt, time.Now().UTC(), passwordMaxAge) {
			http.Error(w, "Lozinka je istekla. Zatražite reset lozinke putem emaila.", http.StatusForbidden)
			return
		}

		consumed, err := users.ConsumeMagicLink(ctx, user.ID, user.MagicLinkTokenHash)
		if err != nil {
			http.Error(w, "Prijava nije uspela", http.StatusInternalServerError)
			return
		}
		if !consumed {
			http.Error(w, "Link za prijavu nije ispravan ili je istekao", http.StatusUnauthorized)
			return
		}

		accessToken, expiresAt, err := IssueAccessToken(user, jwtSecret, tokenValidity)
		if err != nil {
			http.Error(w, "Izdavanje tokena nije uspelo", http.StatusInternalServerError)
			return
		}
		log.Printf("prijava magicnim linkom: korisnik %s", user.ID.Hex())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": accessToken,
			"token_type":   "Bearer",
			"expires_at":   expiresAt,
		})
	})
}
