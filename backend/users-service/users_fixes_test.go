package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestOTPAttemptsPolicy(t *testing.T) {
	for attempts := 0; attempts < maxOTPAttempts; attempts++ {
		if otpAttemptsExhausted(attempts) {
			t.Fatalf("posle %d promasaja kod ne sme jos biti ponisten", attempts)
		}
	}
	if !otpAttemptsExhausted(maxOTPAttempts) {
		t.Fatalf("posle %d promasaja kod mora biti ponisten", maxOTPAttempts)
	}
}

func TestParseUsersPath(t *testing.T) {
	cases := []struct {
		path       string
		id, action string
		ok         bool
	}{
		{"/users/me/quota", "me", "quota", true},
		{"/users/507f1f77bcf86cd799439011/storage", "507f1f77bcf86cd799439011", "storage", true},
		{"/users/abc", "", "", false},
		{"/users//quota", "", "", false},
		{"/users/abc/quota/extra", "", "", false},
		{"/druga/abc/quota", "", "", false},
	}
	for _, c := range cases {
		id, action, ok := parseUsersPath(c.path)
		if id != c.id || action != c.action || ok != c.ok {
			t.Errorf("%s -> (%q, %q, %v)", c.path, id, action, ok)
		}
	}
}

func TestValidateStorageDelta(t *testing.T) {
	for _, ok := range []int64{1, -1, 52428800, -52428800, maxStorageDelta, -maxStorageDelta} {
		if err := validateStorageDelta(ok); err != nil {
			t.Errorf("delta %d je trebalo da prodje: %v", ok, err)
		}
	}
	for _, bad := range []int64{0, maxStorageDelta + 1, -maxStorageDelta - 1} {
		if validateStorageDelta(bad) == nil {
			t.Errorf("delta %d je trebalo da bude odbijena", bad)
		}
	}
}

// Rute koje ne zahtevaju bazu: pogresan metod, nepoznata akcija, tudji ID.
func TestUserQuotaHandlerRoutingAndAuthorization(t *testing.T) {
	h := UserQuotaHandler(nil) // ove grane ne dolaze do repozitorijuma
	own := primitive.NewObjectID().Hex()
	other := primitive.NewObjectID().Hex()

	cases := []struct {
		method, path, userID, body string
		want                       int
	}{
		{http.MethodPost, "/users/" + own + "/quota", own, "", http.StatusMethodNotAllowed},
		{http.MethodGet, "/users/" + own + "/storage", own, "", http.StatusMethodNotAllowed},
		{http.MethodGet, "/users/" + own + "/nesto", own, "", http.StatusNotFound},
		{http.MethodGet, "/users/" + own + "/quota", "", "", http.StatusUnauthorized},
		{http.MethodGet, "/users/" + other + "/quota", own, "", http.StatusForbidden},
		// zauzece sme da se menja samo za korisnika u cije ime se poziva
		{http.MethodPost, "/users/" + other + "/storage", own, `{"delta":5}`, http.StatusForbidden},
		{http.MethodPost, "/users/nije-id/storage", own, `{"delta":5}`, http.StatusBadRequest},
		{http.MethodPost, "/users/" + own + "/storage", own, `{"delta":0}`, http.StatusBadRequest},
		{http.MethodPost, "/users/" + own + "/storage", own, `{"delta":5,"used_storage":0}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
		if c.userID != "" {
			req.Header.Set("X-User-ID", c.userID)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s %s: status %d, ocekivano %d (%s)", c.method, c.path, rec.Code, c.want, strings.TrimSpace(rec.Body.String()))
		}
	}
}

func TestMagicLinkHandlersValidateInputWithoutDatabase(t *testing.T) {
	request := RequestMagicLinkHandler(nil, nil, "http://localhost:4200", 0)
	verify := VerifyMagicLinkHandler(nil, "tajna", 0, 0)

	cases := []struct {
		name string
		h    http.Handler
		m    string
		body string
		want int
	}{
		{"zahtev GET", request, http.MethodGet, "", http.StatusMethodNotAllowed},
		{"zahtev bez mejla", request, http.MethodPost, `{"email":"  "}`, http.StatusBadRequest},
		{"zahtev sa visak polja", request, http.MethodPost, `{"email":"a@b.c","admin":true}`, http.StatusBadRequest},
		{"verifikacija GET", verify, http.MethodGet, "", http.StatusMethodNotAllowed},
		{"neispravan token", verify, http.MethodPost, `{"token":"nije-token"}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		c.h.ServeHTTP(rec, httptest.NewRequest(c.m, "/magic-link", strings.NewReader(c.body)))
		if rec.Code != c.want {
			t.Errorf("%s: status %d, ocekivano %d", c.name, rec.Code, c.want)
		}
	}
}

func TestDefaultLinksPointToClientApp(t *testing.T) {
	t.Setenv("APP_BASE_URL", "")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mail.AppURL != "http://localhost:4200" {
		t.Fatalf("linkovi iz mejlova vode na %q", cfg.Mail.AppURL)
	}
	if cfg.MagicLinkValidity.Minutes() != 15 {
		t.Fatalf("magicni link vazi %v", cfg.MagicLinkValidity)
	}
}
