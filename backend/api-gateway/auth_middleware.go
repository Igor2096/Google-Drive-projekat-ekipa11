package main

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type UserClaims struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

func authMiddleware(jwtSecret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		authHeader := r.Header.Get("Authorization")

		if authHeader == "" {
			http.Error(w, "Nedostaje token za autentifikaciju", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)

		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			http.Error(w, "Neispravan format tokena", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimSpace(parts[1])

		claims := &UserClaims{}

		token, err := jwt.ParseWithClaims(
			tokenString,
			claims,
			func(token *jwt.Token) (interface{}, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			},
		)

		if err != nil || !token.Valid || claims.Subject == "" {
			http.Error(w, "Nevažeći ili istekao token", http.StatusUnauthorized)
			return
		}

		// Gateway uklanja eventualni ID koji je poslao klijent.
		r.Header.Del("X-User-ID")

		// ID korisnika dolazi isključivo iz proverenog JWT tokena.
		r.Header.Set("X-User-ID", claims.Subject)

		next.ServeHTTP(w, r)
	})
}
