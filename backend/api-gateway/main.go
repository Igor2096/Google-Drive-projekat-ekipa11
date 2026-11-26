// API gateway - jedina ulazna tacka u sistem (zahtev 2.2).
//
// Odgovornosti:
//   - prosledjuje zahteve odgovarajucem mikroservisu
//   - proverava JWT i iz njega postavlja X-User-ID / X-User-Role (2.17)
//   - ogranicava broj zahteva po korisniku, zastita od DoS-a (2.17)
//   - HTTPS prema klijentskoj aplikaciji (2.19)
package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type config struct {
	Port             string
	UsersURL         string
	FilesURL         string
	NotificationsURL string
	JWTSecret        string
	AllowedOrigins   []string
	RatePerSecond    float64
	RateBurst        float64
	TLSEnabled       bool
	TLSCertFile      string
	TLSKeyFile       string
}

func loadConfig() config {
	return config{
		Port:             getEnv("GATEWAY_PORT", "8080"),
		UsersURL:         requireEnv("USERS_SERVICE_URL"),
		FilesURL:         requireEnv("FILES_SERVICE_URL"),
		NotificationsURL: requireEnv("NOTIFICATIONS_SERVICE_URL"),
		JWTSecret:        requireEnv("JWT_SECRET"),
		AllowedOrigins:   splitList(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:4200")),
		RatePerSecond:    getFloat("RATE_LIMIT_PER_SECOND", 10),
		RateBurst:        getFloat("RATE_LIMIT_BURST", 30),
		TLSEnabled:       getEnv("TLS_ENABLED", "false") == "true",
		TLSCertFile:      getEnv("TLS_CERT_FILE", "certs/gateway.crt"),
		TLSKeyFile:       getEnv("TLS_KEY_FILE", "certs/gateway.key"),
	}
}

func main() {
	// Lokalni rad (go run) cita .env iz korena projekta; u Docker-u ga nema.
	_ = godotenv.Load("../../.env")
	cfg := loadConfig()

	handler := newRouter(cfg)
	handler = rateLimitMiddleware(newRateLimiter(cfg.RatePerSecond, cfg.RateBurst), cfg.JWTSecret, handler)
	handler = corsMiddleware(cfg.AllowedOrigins, handler)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
		// Vremenska ogranicenja su takodje zastita od DoS-a: bez njih spor
		// klijent (npr. "Slowloris") moze beskonacno da drzi konekcije otvorene.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       2 * time.Minute, // otpremanje vecih datoteka
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}

	if cfg.TLSEnabled {
		if err := ensureCertificate(cfg.TLSCertFile, cfg.TLSKeyFile); err != nil {
			log.Fatalf("TLS sertifikat: %v", err)
		}
		log.Printf("API gateway slusa na https://localhost:%s", cfg.Port)
		log.Fatal(srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile))
	}

	log.Printf("API gateway slusa na http://localhost:%s (TLS iskljucen)", cfg.Port)
	log.Fatal(srv.ListenAndServe())
}

// newRouter povezuje javne i zasticene rute sa servisima.
func newRouter(cfg config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("API gateway radi"))
	})

	users := createProxy(cfg.UsersURL, "/api/users")

	// Javne rute Users servisa: registracija, potvrda mejla, prijava, OTP,
	// reset lozinke i magicni link. Sve sto nije navedeno ispod je javno.
	mux.Handle("/api/users/", users)

	// Kvota korisnika: samo CITANJE. Upis zauzeca (POST /users/{id}/storage)
	// sme da pozove iskljucivo Files servis iznutra; da je propusten ovde,
	// korisnik bi sam sebi mogao da "obrise" zauzeto skladiste.
	mux.Handle("/api/users/users/", onlyMethod(http.MethodGet, authMiddleware(cfg.JWTSecret, users)))

	quotaRequests := authMiddleware(cfg.JWTSecret, users)
	mux.Handle("/api/users/quota-requests", quotaRequests)
	mux.Handle("/api/users/quota-requests/", quotaRequests)

	admin := adminAuthMiddleware(cfg.JWTSecret, users)
	mux.Handle("/api/users/admin/users", admin)
	mux.Handle("/api/users/admin/users/", admin)
	mux.Handle("/api/users/admin/quota-requests", admin)
	mux.Handle("/api/users/admin/quota-requests/", admin)

	mux.Handle("/api/files/", authMiddleware(cfg.JWTSecret, createProxy(cfg.FilesURL, "/api/files")))
	mux.Handle("/api/notifications/", authMiddleware(cfg.JWTSecret, createProxy(cfg.NotificationsURL, "/api/notifications")))

	return mux
}

func createProxy(serviceURL, prefix string) http.Handler {
	target, err := url.Parse(serviceURL)
	if err != nil {
		log.Fatalf("neispravan URL servisa %q: %v", serviceURL, err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("servis %s nije dostupan: %v", target.Host, err)
		http.Error(w, "Servis trenutno nije dostupan", http.StatusBadGateway)
	}
	return http.StripPrefix(prefix, proxy)
}

// onlyMethod propusta samo jedan HTTP metod.
func onlyMethod(method string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			http.Error(w, "Metod nije dozvoljen", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("promenljiva %s nije postavljena", key)
	}
	return v
}

func getFloat(key string, fallback float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil && v > 0 {
		return v
	}
	return fallback
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
