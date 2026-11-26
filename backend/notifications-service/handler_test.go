package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	byUser   map[string][]Notification
	err      error
	askedFor string
}

func (f *fakeStore) ForUser(_ context.Context, userID string) ([]Notification, error) {
	f.askedFor = userID
	if f.err != nil {
		return nil, f.err
	}
	if n, ok := f.byUser[userID]; ok {
		return n, nil
	}
	return []Notification{}, nil
}

const (
	ana   = "507f1f77bcf86cd799439011"
	marko = "507f1f77bcf86cd799439022"
)

func get(t *testing.T, h http.Handler, method, url, userID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, url, nil)
	if userID != "" {
		req.Header.Set(userIDHeader, userID)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestReturnsNotificationsOfLoggedInUser(t *testing.T) {
	store := &fakeStore{byUser: map[string][]Notification{
		ana: {{UserID: ana, NotificationID: "1", Message: "zdravo", CreatedAt: time.Now()}},
	}}
	rec := get(t, notificationsHandler(store), http.MethodGet, "/notifications", ana)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var got []Notification
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Message != "zdravo" {
		t.Fatalf("dobijeno %+v", got)
	}
}

// Zastita od citanja tudjih notifikacija: parametar u URL-u se ignorise,
// vazi samo identitet koji je postavio gateway.
func TestIgnoresUserIDFromQuery(t *testing.T) {
	store := &fakeStore{byUser: map[string][]Notification{marko: {{Message: "tajna"}}}}
	get(t, notificationsHandler(store), http.MethodGet, "/notifications?user_id="+marko, ana)

	if store.askedFor != ana {
		t.Fatalf("servis je citao notifikacije za %q umesto za prijavljenog korisnika", store.askedFor)
	}
}

func TestEmptyListIsArrayNotNull(t *testing.T) {
	rec := get(t, notificationsHandler(&fakeStore{}), http.MethodGet, "/notifications", ana)
	if body := rec.Body.String(); body != "[]\n" {
		t.Fatalf("ocekivan prazan niz, dobijeno %q", body)
	}
}

func TestRejectsMissingOrInvalidUser(t *testing.T) {
	for _, id := range []string{"", "11111111-1111-1111-1111-111111111111", "nije-id", "' OR 1=1 --"} {
		rec := get(t, notificationsHandler(&fakeStore{}), http.MethodGet, "/notifications", id)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("ID %q: status %d, ocekivano 401", id, rec.Code)
		}
	}
}

func TestOnlyGetIsAllowed(t *testing.T) {
	rec := get(t, notificationsHandler(&fakeStore{}), http.MethodPost, "/notifications", ana)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestDatabaseErrorDoesNotLeakDetails(t *testing.T) {
	store := &fakeStore{err: errors.New("gocql: no hosts available in the pool")}
	rec := get(t, notificationsHandler(store), http.MethodGet, "/notifications", ana)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rec.Code)
	}
	if !json.Valid(rec.Body.Bytes()) || strings.Contains(rec.Body.String(), "gocql") {
		t.Fatalf("interni detalji su procureli klijentu: %s", rec.Body.String())
	}
}
