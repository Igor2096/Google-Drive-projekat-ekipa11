package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"files-service/internal/config"
	"files-service/internal/db"
	"files-service/internal/handler"
	"files-service/internal/middleware"
	"files-service/internal/repository"
	"files-service/internal/service"
)

func main() {
	cfg := config.Load()

	client, err := db.Connect(cfg.MongoURI)
	if err != nil {
		log.Fatalf("ne mogu da se povezem na MongoDB: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()
	log.Printf("povezan na MongoDB, baza %q", cfg.MongoDB)

	database := client.Database(cfg.MongoDB)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := db.EnsureIndexes(ctx, database); err != nil {
		log.Fatalf("kreiranje indeksa: %v", err)
	}
	cancel()

	folderRepo := repository.NewFolderRepository(database, cfg.DBTimeout)
	fileRepo := repository.NewFileRepository(database, cfg.DBTimeout)
	folderSvc := service.NewFolderService(folderRepo, fileRepo)

	// Rute koje zahtevaju korisnika
	api := http.NewServeMux()
	handler.NewFolderHandler(folderSvc).Register(api)

	root := http.NewServeMux()
	root.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	root.Handle("/", middleware.Auth(api))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           middleware.Logging(middleware.CORS(cfg.AllowedOrigins)(root)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("files-service slusa na portu %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	// Graceful shutdown (Ctrl+C / docker stop)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("gasim servis...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}
