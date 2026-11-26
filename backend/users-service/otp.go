package main

import (
	"crypto/rand"
	"math/big"
)

const otpLength = 6

const maxOTPAttempts = 5

func otpAttemptsExhausted(attempts int) bool {
	return attempts >= maxOTPAttempts
}

func GenerateOTP() (string, error) {
	code := make([]byte, otpLength)
	for index := range code {
		digit, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code[index] = byte('0' + digit.Int64())
	}
	return string(code), nil
}
