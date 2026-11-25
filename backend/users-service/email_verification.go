package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GenerateEmailVerificationToken(userID primitive.ObjectID) (string, string, error) {
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", err
	}
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)
	return userID.Hex() + "." + secret, secret, nil
}

func ParseEmailVerificationToken(token string) (primitive.ObjectID, string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[1] == "" {
		return primitive.NilObjectID, "", errors.New("neispravan token")
	}
	userID, err := primitive.ObjectIDFromHex(parts[0])
	if err != nil {
		return primitive.NilObjectID, "", errors.New("neispravan token")
	}
	return userID, parts[1], nil
}
