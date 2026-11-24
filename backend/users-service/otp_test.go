package main

import (
	"testing"
	"unicode"
)

func TestGenerateOTP(t *testing.T) {
	otp, err := GenerateOTP()
	if err != nil {
		t.Fatalf("generisanje OTP-a nije uspelo: %v", err)
	}
	if len(otp) != otpLength {
		t.Fatalf("OTP mora imati %d cifara, ima %d", otpLength, len(otp))
	}
	for _, character := range otp {
		if !unicode.IsDigit(character) {
			t.Fatalf("OTP sadrži karakter koji nije cifra: %q", character)
		}
	}
}
