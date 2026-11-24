package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func UserQuotaHandler(users *UserRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		const prefix = "/users/"
		path := strings.TrimPrefix(r.URL.Path, prefix)
		parts := strings.Split(path, "/")
		if !strings.HasPrefix(r.URL.Path, prefix) || len(parts) != 2 || parts[0] == "" || parts[1] != "quota" {
			http.NotFound(w, r)
			return
		}

		userID, err := primitive.ObjectIDFromHex(parts[0])
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
	})
}
