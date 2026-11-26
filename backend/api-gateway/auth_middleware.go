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

func parseUserClaims(jwtSecret string, r *http.Request) (*UserClaims, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, jwt.ErrTokenUnverifiable
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 ||
		!strings.EqualFold(parts[0], "Bearer") ||
		strings.TrimSpace(parts[1]) == "" {
		return nil, jwt.ErrTokenMalformed
	}

	claims := &UserClaims{}

	token, err := jwt.ParseWithClaims(
		strings.TrimSpace(parts[1]),
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecret), nil
		},
	)

	if err != nil || !token.Valid || claims.Subject == "" {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}

func setVerifiedUserHeaders(r *http.Request, claims *UserClaims) {
	// Uklanjamo vrednosti koje je eventualno poslao klijent.
	r.Header.Del("X-User-ID")
	r.Header.Del("X-User-Role")

	// Postavljamo vrednosti iz verifikovanog JWT tokena.
	r.Header.Set("X-User-ID", claims.Subject)
	r.Header.Set("X-User-Role", claims.Role)
}

func authMiddleware(jwtSecret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := parseUserClaims(jwtSecret, r)
		if err != nil {
			http.Error(w, "Nevažeći ili istekao token", http.StatusUnauthorized)
			return
		}

		setVerifiedUserHeaders(r, claims)
		next.ServeHTTP(w, r)
	})
}

// adminAuthMiddleware dozvoljava pristup samo administratorima.
func adminAuthMiddleware(jwtSecret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := parseUserClaims(jwtSecret, r)
		if err != nil {
			http.Error(w, "Nevažeći ili istekao token", http.StatusUnauthorized)
			return
		}

		if claims.Role != "admin" {
			http.Error(w, "Nemate administratorska ovlašćenja", http.StatusForbidden)
			return
		}

		setVerifiedUserHeaders(r, claims)
		next.ServeHTTP(w, r)
	})
}
