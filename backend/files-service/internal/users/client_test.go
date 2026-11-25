package users

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQuotaSuccess(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"storage_quota":1000,"used_storage":400,"available_storage":600}`))
	}))
	defer srv.Close()

	q, err := NewClient(srv.URL, time.Second).Quota(context.Background(), "507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/users/507f1f77bcf86cd799439011/quota" {
		t.Fatalf("pogresna putanja: %s", gotPath)
	}
	if q.StorageQuota != 1000 || q.UsedStorage != 400 || q.AvailableStorage != 600 {
		t.Fatalf("pogresno procitana kvota: %+v", q)
	}
}

func TestQuotaUserNotFound(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusBadRequest} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "nema ga", status)
		}))
		_, err := NewClient(srv.URL, time.Second).Quota(context.Background(), "x")
		srv.Close()
		if !errors.Is(err, ErrUserNotFound) {
			t.Errorf("status %d: ocekivano ErrUserNotFound, dobijeno %v", status, err)
		}
	}
}

func TestQuotaServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "puklo", http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, time.Second).Quota(context.Background(), "x"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ocekivano ErrUnavailable, dobijeno %v", err)
	}
}

func TestQuotaServiceDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // servis vise ne slusa

	if _, err := NewClient(url, 200*time.Millisecond).Quota(context.Background(), "x"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ocekivano ErrUnavailable, dobijeno %v", err)
	}
}

func TestQuotaGarbageResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ovo nije JSON"))
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, time.Second).Quota(context.Background(), "x"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ocekivano ErrUnavailable, dobijeno %v", err)
	}
}
