package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// maxStorageDelta - gornja granica jedne promene zauzeca (1 TiB).
// Provera opsega ulaza (boundary checking, 2.18): odbija besmislene vrednosti
// i stiti od prekoracenja int64 pri sabiranju.
const maxStorageDelta int64 = 1 << 40

// parseUsersPath rasclanjuje /users/{id}/{akcija}. id moze biti i "me".
func parseUsersPath(path string) (id, action string, ok bool) {
	const prefix = "/users/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// UserQuotaHandler opsluzuje:
//
//	GET  /users/{id}/quota    - kvota i zauzece (korisnik za sebe, admin za sve)
//	POST /users/{id}/storage  - promena zauzeca; poziva je SAMO Files servis
//	                            (gateway ovu rutu ne propusta spolja)
func UserQuotaHandler(users *UserRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, action, ok := parseUsersPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}

		switch action {
		case "quota":
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", http.MethodGet)
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			getQuota(users, w, r, id)
		case "storage":
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", http.MethodPost)
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			addUsedStorage(users, w, r, id)
		default:
			http.NotFound(w, r)
		}
	})
}

func getQuota(users *UserRepository, w http.ResponseWriter, r *http.Request, pathID string) {
	requesterID := r.Header.Get("X-User-ID")
	requesterRole := r.Header.Get("X-User-Role")

	if requesterID == "" {
		http.Error(w, "Niste autentifikovani", http.StatusUnauthorized)
		return
	}

	var userID primitive.ObjectID
	var err error

	if pathID == "me" {
		userID, err = primitive.ObjectIDFromHex(requesterID)
	} else {
		userID, err = primitive.ObjectIDFromHex(pathID)
		if err == nil && requesterRole != "admin" &&
			!strings.EqualFold(requesterID, userID.Hex()) {
			http.Error(w, "Nemate pravo pristupa ovoj kvoti", http.StatusForbidden)
			return
		}
	}

	if err != nil {
		http.Error(w, "Neispravan ID korisnika", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	user, err := users.FindByID(ctx, userID)
	if err != nil {
		http.Error(w, "Korisnik nije pronađen", http.StatusNotFound)
		return
	}
	writeQuota(w, user)
}

type storageChangeRequest struct {
	Delta int64 `json:"delta"`
}

// validateStorageDelta - delta mora biti razlicita od nule i u razumnom opsegu.
func validateStorageDelta(delta int64) error {
	if delta == 0 {
		return errors.New("delta mora biti različita od nule")
	}
	if delta > maxStorageDelta || delta < -maxStorageDelta {
		return errors.New("delta je van dozvoljenog opsega")
	}
	return nil
}

func addUsedStorage(users *UserRepository, w http.ResponseWriter, r *http.Request, pathID string) {
	userID, err := primitive.ObjectIDFromHex(pathID)
	if err != nil {
		http.Error(w, "Neispravan ID korisnika", http.StatusBadRequest)
		return
	}
	// Files servis salje ID vlasnika datoteke; menja se zauzece samo tog korisnika.
	if !strings.EqualFold(r.Header.Get("X-User-ID"), userID.Hex()) {
		http.Error(w, "Nemate pravo da menjate ovo zauzeće", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	defer r.Body.Close()
	var request storageChangeRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "Neispravno telo zahteva", http.StatusBadRequest)
		return
	}
	if err := validateStorageDelta(request.Delta); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	user, err := users.AddUsedStorage(ctx, userID, request.Delta)
	if errors.Is(err, mongo.ErrNoDocuments) {
		http.Error(w, "Korisnik nije pronađen", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Ažuriranje zauzeća nije uspelo", http.StatusInternalServerError)
		return
	}
	writeQuota(w, user)
}

func writeQuota(w http.ResponseWriter, user *User) {
	available := user.StorageQuota - user.UsedStorage
	if available < 0 {
		available = 0
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int64{
		"storage_quota":     user.StorageQuota,
		"used_storage":      user.UsedStorage,
		"available_storage": available,
	})
}
