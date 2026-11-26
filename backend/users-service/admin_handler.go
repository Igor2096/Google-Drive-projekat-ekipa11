package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AdminUserResponse struct {
	ID            string `json:"id"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Email         string `json:"email"`
	Role          string `json:"role"`
	StorageQuota  int64  `json:"storage_quota"`
	UsedStorage   int64  `json:"used_storage"`
	AvailableSize int64  `json:"available_storage"`
}

func AdminUsersHandler(users *UserRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Metoda nije dozvoljena", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()

		allUsers, err := users.FindAll(ctx)
		if err != nil {
			http.Error(w, "Greška pri učitavanju korisnika", http.StatusInternalServerError)
			return
		}

		response := make([]AdminUserResponse, 0, len(allUsers))

		for _, user := range allUsers {
			available := user.StorageQuota - user.UsedStorage
			if available < 0 {
				available = 0
			}

			response = append(response, AdminUserResponse{
				ID:            user.ID.Hex(),
				FirstName:     user.FirstName,
				LastName:      user.LastName,
				Email:         user.Email,
				Role:          user.Role,
				StorageQuota:  user.StorageQuota,
				UsedStorage:   user.UsedStorage,
				AvailableSize: available,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})
}

func AdminQuotaRequestsHandler(
	users *UserRepository,
	requests *QuotaRequestRepository,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/admin/quota-requests"), "/")

		if path == "" {
			if r.Method != http.MethodGet {
				http.Error(w, "Metoda nije dozvoljena", http.StatusMethodNotAllowed)
				return
			}

			allRequests, err := requests.FindAll(r.Context())
			if err != nil {
				http.Error(w, "Greška pri učitavanju zahteva", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(allRequests)
			return
		}

		if r.Method != http.MethodPatch {
			http.Error(w, "Metoda nije dozvoljena", http.StatusMethodNotAllowed)
			return
		}

		requestID, err := primitive.ObjectIDFromHex(path)
		if err != nil {
			http.Error(w, "Neispravan ID zahteva", http.StatusBadRequest)
			return
		}

		var body struct {
			Status string `json:"status"`
		}

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		if err := decoder.Decode(&body); err != nil {
			http.Error(w, "Neispravno telo zahteva", http.StatusBadRequest)
			return
		}

		if body.Status != QuotaRequestApproved && body.Status != QuotaRequestRejected {
			http.Error(w, "Status mora biti approved ili rejected", http.StatusBadRequest)
			return
		}

		quotaRequest, err := requests.FindByID(r.Context(), requestID)
		if err != nil {
			if errors.Is(err, ErrQuotaRequestNotFound) {
				http.Error(w, "Zahtev nije pronađen", http.StatusNotFound)
				return
			}
			http.Error(w, "Greška pri učitavanju zahteva", http.StatusInternalServerError)
			return
		}

		if quotaRequest.Status != QuotaRequestPending {
			http.Error(w, "Zahtev je već obrađen", http.StatusConflict)
			return
		}

		var oldQuota int64

		if body.Status == QuotaRequestApproved {
			user, err := users.FindByID(r.Context(), quotaRequest.UserID)
			if err != nil {
				http.Error(w, "Korisnik zahteva nije pronađen", http.StatusNotFound)
				return
			}

			oldQuota = user.StorageQuota

			if quotaRequest.RequestedQuota <= oldQuota {
				http.Error(w, "Tražena kvota više nije veća od trenutne", http.StatusConflict)
				return
			}

			if err := users.UpdateStorageQuota(
				r.Context(),
				quotaRequest.UserID,
				quotaRequest.RequestedQuota,
			); err != nil {
				http.Error(w, "Greška pri ažuriranju kvote korisnika", http.StatusInternalServerError)
				return
			}
		}

		if err := requests.UpdateStatus(r.Context(), requestID, body.Status); err != nil {
			// Ako ažuriranje statusa ne uspe, pokušavamo da vratimo staru kvotu.
			if body.Status == QuotaRequestApproved {
				_ = users.UpdateStorageQuota(r.Context(), quotaRequest.UserID, oldQuota)
			}

			if errors.Is(err, ErrQuotaRequestNotFound) {
				http.Error(w, "Zahtev je već obrađen ili ne postoji", http.StatusConflict)
				return
			}

			http.Error(w, "Greška pri ažuriranju statusa zahteva", http.StatusInternalServerError)
			return
		}

		quotaRequest.Status = body.Status
		quotaRequest.UpdatedAt = time.Now().UTC()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(quotaRequest)
	})
}
