package main

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// rateLimiter - "token bucket" po kljucu (zahtev 2.17, zastita od DoS-a).
//
// Svaki kljuc (korisnik ili IP adresa) ima kofu sa najvise `burst` zetona.
// Svaki zahtev trosi jedan zeton, a kofa se puni brzinom `rate` zetona u
// sekundi. Kratak nalet zahteva prolazi, ali dugotrajno zatrpavanje ne.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64
	burst   float64
	now     func() time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newRateLimiter(rate, burst float64) *rateLimiter {
	l := &rateLimiter{buckets: map[string]*bucket{}, rate: rate, burst: burst, now: time.Now}
	go l.cleanup()
	return l
}

func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &bucket{tokens: l.burst - 1, last: now}
		return true
	}

	b.tokens += now.Sub(b.last).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// cleanup brise kofe koje dugo nisu koriscene, da mapa ne raste unedogled.
func (l *rateLimiter) cleanup() {
	for range time.Tick(time.Minute) {
		l.mu.Lock()
		for key, b := range l.buckets {
			if l.now().Sub(b.last) > 10*time.Minute {
				delete(l.buckets, key)
			}
		}
		l.mu.Unlock()
	}
}

// rateLimitMiddleware ogranicava po KORISNIKU kada je prijavljen (ID iz
// JWT-a), a po IP adresi za javne rute - tako se pokriva i pogadjanje
// lozinke ili OTP koda na /login i /verify-otp.
func rateLimitMiddleware(l *rateLimiter, jwtSecret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientKey(r, jwtSecret)
		if !l.allow(key) {
			log.Printf("ograniceno: previse zahteva od %s (%s %s)", key, r.Method, r.URL.Path)
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Previse zahteva. Pokusajte ponovo za trenutak.", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientKey(r *http.Request, jwtSecret string) string {
	if claims, err := parseUserClaims(jwtSecret, r); err == nil {
		return "korisnik:" + claims.Subject
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return "ip:" + host
}
