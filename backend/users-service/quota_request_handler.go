package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const UserIDHeader = "X-User-ID"

func CreateQuotaRequestHandler(
	users *UserRepository,
	requests *QuotaRequestRepository,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userIDValue := r.Header.Get(UserIDHeader)
		if userIDValue == "" {
			http.Error(w, "Korisnik nije autentifikovan", http.StatusUnauthorized)
			return
		}

		userID, err := primitive.ObjectIDFromHex(userIDValue)
		if err != nil {
			http.Error(w, "Neispravan ID korisnika", http.StatusUnauthorized)
			return
		}

		var input struct {
			RequestedQuota int64 `json:"requested_quota"`
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		if err := decoder.Decode(&input); err != nil {
			http.Error(w, "Neispravan JSON zahtev", http.StatusBadRequest)
			return
		}

		var extra interface{}
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(w, "Zahtev mora sadržati samo jedan JSON objekat", http.StatusBadRequest)
			return
		}

		if input.RequestedQuota <= 0 {
			http.Error(w, "Tražena kvota mora biti veća od nule", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		user, err := users.FindByID(ctx, userID)
		if err != nil {
			http.Error(w, "Korisnik nije pronađen", http.StatusNotFound)
			return
		}

		if input.RequestedQuota <= user.StorageQuota {
			http.Error(w, "Tražena kvota mora biti veća od trenutne kvote", http.StatusBadRequest)
			return
		}

		now := time.Now().UTC()

		request := &QuotaRequest{
			UserID:         userID,
			RequestedQuota: input.RequestedQuota,
			Status:         QuotaRequestPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		if err := requests.Create(ctx, request); err != nil {
			http.Error(w, "Čuvanje zahteva nije uspelo", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		_ = json.NewEncoder(w).Encode(request)
	})
}
