package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"campusclaw/backend/internal/app"
)

func main() {
	config, err := app.ConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
	store, err := app.OpenStore(ctx, config.DatabaseDSN)
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	defer store.DB.Close()
	if err := store.Migrate(context.Background()); err != nil {
		log.Fatal(err)
	}
	if err := app.RestoreSeedMaterialFiles(context.Background(), store, config.UploadDir); err != nil {
		log.Fatal(err)
	}
	retrieval := app.NewRetrievalService(config, store)
	if err := retrieval.Initialize(context.Background()); err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Addr: config.Addr, Handler: app.NewServer(config, store, retrieval).Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Printf("CampusClaw API listening on %s", config.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
