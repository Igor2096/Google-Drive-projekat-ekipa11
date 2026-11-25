// Package users - HTTP klijent ka Users servisu.
//
// Ovo je jedini nacin na koji Files servis sme da sazna nesto o korisniku:
// baza korisnika pripada Users servisu i ne dira se direktno (zahtev 2.5).
package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var (
	// ErrUserNotFound - Users servis ne zna za tog korisnika.
	ErrUserNotFound = errors.New("korisnik ne postoji")
	// ErrUnavailable - Users servis ne odgovara ili je vratio gresku.
	ErrUnavailable = errors.New("users servis nije dostupan")
)

// Quota je odgovor endpointa GET /users/{id}/quota.
// Nazivi polja prate JSON koji vraca Users servis.
type Quota struct {
	StorageQuota     int64 `json:"storage_quota"`     // ukupno dodeljeno, u bajtovima
	UsedStorage      int64 `json:"used_storage"`      // trenutno zauzeto
	AvailableStorage int64 `json:"available_storage"` // koliko jos staje
}

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout},
	}
}

// Quota dohvata kvotu korisnika sinhrono - Files servis ceka odgovor
// pre nego sto dozvoli otpremanje (zahtev 2.5, task T-23).
func (c *Client) Quota(ctx context.Context, userID string) (Quota, error) {
	var q Quota

	endpoint := fmt.Sprintf("%s/users/%s/quota", c.baseURL, url.PathEscape(userID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return q, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return q, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
			return Quota{}, fmt.Errorf("%w: neocekivan odgovor: %v", ErrUnavailable, err)
		}
		return q, nil
	case http.StatusNotFound, http.StatusBadRequest:
		return q, ErrUserNotFound
	default:
		return q, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
}
