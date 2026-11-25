package main

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestEmailVerificationTokenRoundTrip(t *testing.T) {
	userID := primitive.NewObjectID()
	token, secret, err := GenerateEmailVerificationToken(userID)
	if err != nil {
		t.Fatalf("generisanje tokena nije uspelo: %v", err)
	}
	parsedID, parsedSecret, err := ParseEmailVerificationToken(token)
	if err != nil {
		t.Fatalf("parsiranje tokena nije uspelo: %v", err)
	}
	if parsedID != userID || parsedSecret != secret {
		t.Fatal("token nije moguće pravilno parsirati")
	}
}
