// Notifications servis - cuva notifikacije korisnika u Cassandri (wide-column)
// i vraca ih prijavljenom korisniku (zahtev 1.7).
//
// Pokretanje:
//
//	notifications-service              -> HTTP servis
//	notifications-service -seed <ID>   -> upise probne notifikacije za korisnika
//	                                      sa datim ID-jem i izadje ("rucno
//	                                      popunjavanje baze" za ocenu 6)
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

type config struct {
	Port          string
	CassandraHost string
	CassandraPort int
	Keyspace      string
}

func loadConfig() (config, error) {
	port, err := strconv.Atoi(getEnv("CASSANDRA_PORT", "9042"))
	if err != nil {
		return config{}, errors.New("CASSANDRA_PORT mora biti broj")
	}
	return config{
		Port:          getEnv("NOTIFICATIONS_SERVICE_PORT", "8083"),
		CassandraHost: getEnv("CASSANDRA_HOST", "localhost"),
		CassandraPort: port,
		Keyspace:      getEnv("CASSANDRA_KEYSPACE", "notifications_db"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	seedUser := flag.String("seed", "", "ID korisnika (Mongo ObjectID) za koga se upisuju probne notifikacije")
	flag.Parse()

	// Lokalni rad (go run) cita .env iz korena projekta; u Docker-u ga nema
	// i promenljive dolaze iz docker-compose.yml.
	_ = godotenv.Load("../../.env")

	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	session, err := connect(cfg)
	if err != nil {
		log.Fatalf("Cassandra nije dostupna: %v", err)
	}
	defer session.Close()
	store := &cassandraStore{session: session}

	if *seedUser != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := seedNotifications(ctx, store, *seedUser); err != nil {
			log.Fatalf("seed nije uspeo: %v", err)
		}
		log.Printf("upisane probne notifikacije za korisnika %s", *seedUser)
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/notifications", notificationsHandler(store))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
	}

	go func() {
		log.Printf("Notifications servis slusa na portu %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
