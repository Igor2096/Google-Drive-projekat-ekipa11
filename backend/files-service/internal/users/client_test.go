package users

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQuotaSuccess(t *testing.T) {
	var gotPath, gotUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotUser = r.Header.Get("X-User-ID")
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
	// Users servis odbija citanje kvote bez ovog header-a (401), pa bi
	// svako otpremanje padalo sa 503.
	if gotUser != "507f1f77bcf86cd799439011" {
		t.Fatalf("X-User-ID nije poslat, dobijeno %q", gotUser)
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

func TestAddUsedStorage(t *testing.T) {
	var gotPath, gotMethod, gotBody, gotUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		gotUser = r.Header.Get("X-User-ID")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := NewClient(srv.URL, time.Second).AddUsedStorage(context.Background(), "abc", 1234); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/users/abc/storage" {
		t.Fatalf("%s %s", gotMethod, gotPath)
	}
	if gotBody != `{"delta":1234}` {
		t.Fatalf("telo zahteva: %s", gotBody)
	}
	if gotUser != "abc" {
		t.Fatalf("X-User-ID nije poslat, dobijeno %q", gotUser)
	}
}

func TestAddUsedStorageNegativeDelta(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := NewClient(srv.URL, time.Second).AddUsedStorage(context.Background(), "abc", -500); err != nil {
		t.Fatal(err)
	}
	if gotBody != `{"delta":-500}` {
		t.Fatalf("telo zahteva: %s", gotBody)
	}
}

func TestAddUsedStorageErrors(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusNotFound, ErrUserNotFound},
		{http.StatusInternalServerError, ErrUnavailable},
		{http.StatusMethodNotAllowed, ErrUnavailable},
	}
	for _, c := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(c.status)
		}))
		err := NewClient(srv.URL, time.Second).AddUsedStorage(context.Background(), "abc", 1)
		srv.Close()
		if !errors.Is(err, c.want) {
			t.Errorf("status %d: ocekivano %v, dobijeno %v", c.status, c.want, err)
		}
	}
}
