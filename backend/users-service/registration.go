package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode"
)

const maxRequestBodyBytes = 1 << 20

type registerRequest struct {
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func RegistrationHandler(users *UserRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		defer r.Body.Close()
		var request registerRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			http.Error(w, "Neispravno telo zahteva", http.StatusBadRequest)
			return
		}
		if err := validateRegisterRequest(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		hash, err := HashPassword(request.Password)
		if err != nil {
			http.Error(w, "Greška pri obradi lozinke", http.StatusInternalServerError)
			return
		}
		now := time.Now().UTC()
		user := &User{FirstName: request.FirstName, LastName: request.LastName, Email: request.Email, PasswordHash: hash, Role: regularUserRole, CreatedAt: now, UpdatedAt: now}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := users.Create(ctx, user); err != nil {
			if errors.Is(err, ErrEmailTaken) {
				http.Error(w, "Email je već registrovan", http.StatusConflict)
				return
			}
			http.Error(w, "Registracija nije uspela", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": user.ID.Hex(), "email": user.Email, "message": "Registracija je uspešna"})
	})
}

func validateRegisterRequest(request *registerRequest) error {
	request.FirstName = strings.TrimSpace(request.FirstName)
	request.LastName = strings.TrimSpace(request.LastName)
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	if request.FirstName == "" || request.LastName == "" {
		return errors.New("Ime i prezime su obavezni")
	}
	if len([]rune(request.FirstName)) > 100 || len([]rune(request.LastName)) > 100 {
		return errors.New("Ime i prezime mogu imati najviše 100 karaktera")
	}
	address, err := mail.ParseAddress(request.Email)
	if err != nil || address.Address != request.Email {
		return errors.New("Email adresa nije ispravna")
	}
	if request.Password != request.ConfirmPassword {
		return errors.New("Lozinke se ne podudaraju")
	}
	return validateStrongPassword(request.Password)
}

func validateStrongPassword(password string) error {
	if len([]rune(password)) < 8 {
		return errors.New("Lozinka mora imati najmanje 8 karaktera")
	}
	var upper, lower, number, special bool
	for _, character := range password {
		switch {
		case unicode.IsUpper(character):
			upper = true
		case unicode.IsLower(character):
			lower = true
		case unicode.IsNumber(character):
			number = true
		case unicode.IsPunct(character) || unicode.IsSymbol(character):
			special = true
		}
	}
	if !upper || !lower || !number || !special {
		return errors.New("Lozinka mora sadržati veliko slovo, malo slovo, broj i specijalni karakter")
	}
	return nil
}
