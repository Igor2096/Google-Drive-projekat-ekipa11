package main

import "testing"

func TestValidateRegisterRequest(t *testing.T) {
	request := registerRequest{FirstName: "Mateja", LastName: "Zinaic", Email: "MATEJA@example.com", Password: "JakaLozinka1!", ConfirmPassword: "JakaLozinka1!"}
	if err := validateRegisterRequest(&request); err != nil {
		t.Fatalf("validan zahtev je odbijen: %v", err)
	}
	if request.Email != "mateja@example.com" {
		t.Fatalf("email nije normalizovan: %q", request.Email)
	}
	request.Password, request.ConfirmPassword = "slaba", "slaba"
	if err := validateRegisterRequest(&request); err == nil {
		t.Fatal("slaba lozinka mora biti odbijena")
	}
}
