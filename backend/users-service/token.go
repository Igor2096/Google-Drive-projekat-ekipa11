package main

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type userClaims struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

func IssueAccessToken(user *User, secret string, validity time.Duration) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(validity)
	claims := userClaims{
		Email: user.Email,
		Role:  user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.Hex(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	return signed, expiresAt, err
}
