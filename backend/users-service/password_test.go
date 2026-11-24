package main

import "testing"

func TestHashPassword(t *testing.T) {
	password := "JakaLozinka1!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hashiranje nije uspelo: %v", err)
	}
	if hash == password {
		t.Fatal("hash ne sme biti jednak originalnoj lozinci")
	}
	if err := CheckPassword(hash, password); err != nil {
		t.Fatalf("verifikacija lozinke nije uspela: %v", err)
	}
}
