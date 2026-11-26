package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-tajna"

func tokenFor(t *testing.T, userID, role string) string {
	t.Helper()
	claims := UserClaims{Role: role, RegisteredClaims: jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

// --- ogranicenje broja zahteva ---

func TestRateLimiterBlocksAfterBurstAndRefills(t *testing.T) {
	now := time.Unix(1000, 0)
	l := &rateLimiter{buckets: map[string]*bucket{}, rate: 1, burst: 3, now: func() time.Time { return now }}

	for i := 0; i < 3; i++ {
		if !l.allow("ana") {
			t.Fatalf("zahtev %d je trebalo da prodje", i+1)
		}
	}
	if l.allow("ana") {
		t.Fatal("cetvrti zahtev odmah posle naleta je trebalo da bude odbijen")
	}
	if !l.allow("marko") {
		t.Fatal("drugi korisnik ne sme da trpi zbog prvog")
	}

	now = now.Add(time.Second) // prosla sekunda -> jedan nov zeton
	if !l.allow("ana") {
		t.Fatal("posle sekunde je trebalo da stigne nov zeton")
	}
	if l.allow("ana") {
		t.Fatal("ali samo jedan")
	}
}

func TestRateLimitMiddlewareReturns429(t *testing.T) {
	l := &rateLimiter{buckets: map[string]*bucket{}, rate: 0.001, burst: 2, now: time.Now}
	h := rateLimitMiddleware(l, testSecret, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))

	codes := []int{}
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users/login", nil))
		codes = append(codes, rec.Code)
	}
	if codes[2] != http.StatusTooManyRequests {
		t.Fatalf("statusi %v, treci je trebalo da bude 429", codes)
	}
}

// Prijavljen korisnik se ogranicava po svom ID-ju, ne po IP adresi.
func TestClientKeyUsesUserIDWhenLoggedIn(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/files/x", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, "507f1f77bcf86cd799439011", "user"))
	if got := clientKey(req, testSecret); got != "korisnik:507f1f77bcf86cd799439011" {
		t.Fatalf("kljuc %q", got)
	}
	anon := httptest.NewRequest(http.MethodGet, "/api/users/login", nil)
	if got := clientKey(anon, testSecret); got != "ip:192.0.2.1" {
		t.Fatalf("kljuc %q", got)
	}
}

// --- rute ---

func testRouter(t *testing.T) (http.Handler, *string, *http.Header) {
	t.Helper()
	var lastPath string
	var lastHeaders http.Header
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.Method + " " + r.URL.Path
		lastHeaders = r.Header.Clone()
	}))
	t.Cleanup(backend.Close)
	cfg := config{UsersURL: backend.URL, FilesURL: backend.URL, NotificationsURL: backend.URL, JWTSecret: testSecret}
	return newRouter(cfg), &lastPath, &lastHeaders
}

func do(h http.Handler, method, path, token string) int {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

// Upis zauzeca skladista ne sme biti dostupan spolja, cak ni prijavljenom korisniku.
func TestStorageUpdateIsNotReachableFromOutside(t *testing.T) {
	h, _, _ := testRouter(t)
	tok := tokenFor(t, "507f1f77bcf86cd799439011", "user")

	if code := do(h, http.MethodPost, "/api/users/users/507f1f77bcf86cd799439011/storage", tok); code != http.StatusMethodNotAllowed {
		t.Fatalf("POST na storage: status %d, ocekivano 405", code)
	}
	if code := do(h, http.MethodGet, "/api/users/users/me/quota", tok); code != http.StatusOK {
		t.Fatalf("GET kvote: status %d", code)
	}
}

func TestProtectedRoutesRequireToken(t *testing.T) {
	h, _, _ := testRouter(t)
	for _, path := range []string{"/api/files/folders/root/contents", "/api/notifications/notifications", "/api/users/users/me/quota", "/api/users/admin/users"} {
		if code := do(h, http.MethodGet, path, ""); code != http.StatusUnauthorized {
			t.Errorf("%s bez tokena: status %d", path, code)
		}
	}
}

func TestAdminRoutesRequireAdminRole(t *testing.T) {
	h, _, _ := testRouter(t)
	if code := do(h, http.MethodGet, "/api/users/admin/users", tokenFor(t, "507f1f77bcf86cd799439011", "user")); code != http.StatusForbidden {
		t.Fatalf("obican korisnik na admin ruti: status %d", code)
	}
	if code := do(h, http.MethodGet, "/api/users/admin/users", tokenFor(t, "507f1f77bcf86cd799439011", "admin")); code != http.StatusOK {
		t.Fatalf("admin: status %d", code)
	}
}

// Gateway mora da prepise X-User-ID koji je klijent podmetnuo.
func TestForgedUserHeaderIsReplaced(t *testing.T) {
	h, path, headers := testRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/files/folders/root/contents", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, "507f1f77bcf86cd799439011", "user"))
	req.Header.Set("X-User-ID", "507f1f77bcf86cd799439099") // tudji ID
	h.ServeHTTP(httptest.NewRecorder(), req)

	if *path != "GET /folders/root/contents" {
		t.Fatalf("prosledjeno na %q", *path)
	}
	if got := headers.Get("X-User-ID"); got != "507f1f77bcf86cd799439011" {
		t.Fatalf("servis je dobio X-User-ID %q", got)
	}
}

func TestPublicUserRoutesNeedNoToken(t *testing.T) {
	h, path, _ := testRouter(t)
	for _, p := range []string{"/api/users/login", "/api/users/register", "/api/users/magic-link", "/api/users/confirm-email"} {
		if code := do(h, http.MethodPost, p, ""); code != http.StatusOK {
			t.Errorf("%s: status %d", p, code)
		}
	}
	if *path != "POST /confirm-email" {
		t.Fatalf("poslednje prosledjeno: %q", *path)
	}
}

// --- CORS ---

func TestCORSOnlyForAllowedOrigins(t *testing.T) {
	h := corsMiddleware([]string{"http://localhost:4200"}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	ok := httptest.NewRequest(http.MethodOptions, "/api/files/x", nil)
	ok.Header.Set("Origin", "http://localhost:4200")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, ok)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:4200" {
		t.Fatal("dozvoljen origin nije dobio CORS zaglavlje")
	}

	evil := httptest.NewRequest(http.MethodOptions, "/api/files/x", nil)
	evil.Header.Set("Origin", "https://napadac.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, evil)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("nepoznat origin ne sme dobiti CORS zaglavlje")
	}
}

// --- TLS ---

func TestEnsureCertificateCreatesUsableCertOnce(t *testing.T) {
	dir := t.TempDir()
	cert, key := filepath.Join(dir, "c", "gateway.crt"), filepath.Join(dir, "c", "gateway.key")

	if err := ensureCertificate(cert, key); err != nil {
		t.Fatal(err)
	}
	if _, err := tls.LoadX509KeyPair(cert, key); err != nil {
		t.Fatalf("sertifikat nije upotrebljiv: %v", err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(key)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("privatni kljuc ima prava %v, ocekivano 0600", info.Mode().Perm())
		}
	}

	before, _ := os.ReadFile(cert)
	if err := ensureCertificate(cert, key); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(cert)
	if string(before) != string(after) {
		t.Fatal("postojeci sertifikat ne sme da se pregazi pri svakom pokretanju")
	}
}
