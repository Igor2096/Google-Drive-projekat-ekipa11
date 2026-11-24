package main

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestIssueAccessToken(t *testing.T) {
	user := &User{ID: primitive.NewObjectID(), Email: "mateja@example.com", Role: regularUserRole}
	token, expiresAt, err := IssueAccessToken(user, "test-secret", time.Hour)
	if err != nil {
		t.Fatalf("izdavanje tokena nije uspelo: %v", err)
	}
	if !expiresAt.After(time.Now()) {
		t.Fatal("token mora imati buduće vreme isteka")
	}
	parsed, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) { return []byte("test-secret"), nil })
	if err != nil || !parsed.Valid {
		t.Fatalf("token nije validan: %v", err)
	}
}
