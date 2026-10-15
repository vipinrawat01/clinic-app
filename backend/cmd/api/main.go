package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omnia/clinic-service/internal/auth"
	"github.com/omnia/clinic-service/internal/config"
	"github.com/omnia/clinic-service/internal/exceptions"
	clinichttp "github.com/omnia/clinic-service/internal/http"
	"github.com/omnia/clinic-service/internal/transport"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}

	issuer := auth.NewTokenIssuer(cfg.JWTSecret)
	transportClient := transport.New(cfg.MainPlatformBaseURL, cfg.MainPlatformAPIKey, cfg.MainPlatformAgentID, cfg.TransportCoordinatorNumber)

	router := clinichttp.NewRouter(clinichttp.Dependencies{
		Pool: pool, Issuer: issuer, ServiceKey: cfg.ServiceKey, AllowedOrigins: cfg.AllowedOrigins,
		Transport: transportClient,
	})

	// Periodic sweep for operational exceptions (no-shows, delay cascades)
	// — see internal/exceptions.
	exceptionSweeper := &exceptions.Runner{Pool: pool}
	go exceptionSweeper.Start(ctx, 2*time.Minute)

	log.Printf("clinic-service listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
