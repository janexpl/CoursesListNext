package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/config"
	"github.com/janexpl/CoursesListNext/api/internal/db"
	dbsql "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/server"
	"github.com/janexpl/CoursesListNext/api/internal/webhooks"
)

// expiredSessionPurger deletes expired session rows; *dbsql.Queries satisfies it.
type expiredSessionPurger interface {
	DeleteExpiredSessions(ctx context.Context) (int64, error)
}

// idempotencyKeyRetention - jak długo ponowienie z tym samym kluczem idempotencji
// zwraca pierwotne zaświadczenie. Kontrakt API gwarantuje co najmniej 30 dni.
const idempotencyKeyRetention = 30 * 24 * time.Hour

// staleIdempotencyKeyPurger deletes idempotency keys past retention; *dbsql.Queries satisfies it.
type staleIdempotencyKeyPurger interface {
	DeleteIdempotencyKeysOlderThan(ctx context.Context, cutoff pgtype.Timestamptz) (int64, error)
}

func main() {
	cfg := config.Load()
	pool, err := db.NewConnection(&cfg)
	if err != nil {
		log.Fatalln("Unable to connect database:", err)
	}
	defer pool.Close()
	queries := dbsql.New(pool)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.NewRouter(server.Dependencies{Queries: queries, Config: &cfg, Pool: pool}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go startSessionCleanup(ctx, queries, cfg.SessionCleanupInterval)
	go startIdempotencyKeyCleanup(ctx, queries, cfg.SessionCleanupInterval)
	if cfg.WebhooksEnabled {
		go webhooks.NewDispatcher(pool, webhooks.DefaultConfig()).Run(ctx)
	}

	go func() {
		log.Printf("api listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down api")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

// startSessionCleanup periodically removes expired sessions until ctx is done,
// running once promptly at startup.
func startSessionCleanup(ctx context.Context, purger expiredSessionPurger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	purgeExpiredSessions(ctx, purger)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			purgeExpiredSessions(ctx, purger)
		}
	}
}

func purgeExpiredSessions(ctx context.Context, purger expiredSessionPurger) {
	deleted, err := purger.DeleteExpiredSessions(ctx)
	if err != nil {
		log.Printf("failed to delete expired sessions: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("deleted %d expired sessions", deleted)
	}
}

// startIdempotencyKeyCleanup periodically removes idempotency keys older than
// idempotencyKeyRetention until ctx is done, running once promptly at startup.
func startIdempotencyKeyCleanup(ctx context.Context, purger staleIdempotencyKeyPurger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	purgeStaleIdempotencyKeys(ctx, purger, time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			purgeStaleIdempotencyKeys(ctx, purger, now)
		}
	}
}

func purgeStaleIdempotencyKeys(ctx context.Context, purger staleIdempotencyKeyPurger, now time.Time) {
	deleted, err := purger.DeleteIdempotencyKeysOlderThan(ctx, pgtype.Timestamptz{
		Time:  now.Add(-idempotencyKeyRetention),
		Valid: true,
	})
	if err != nil {
		log.Printf("failed to delete stale idempotency keys: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("deleted %d stale idempotency keys", deleted)
	}
}
