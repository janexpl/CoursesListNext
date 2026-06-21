package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/janexpl/CoursesListNext/api/internal/config"
	"github.com/janexpl/CoursesListNext/api/internal/db"
	dbsql "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/server"
)

const sessionCleanupInterval = time.Hour

// expiredSessionPurger deletes expired session rows; *dbsql.Queries satisfies it.
type expiredSessionPurger interface {
	DeleteExpiredSessions(ctx context.Context) (int64, error)
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

	go startSessionCleanup(ctx, queries, sessionCleanupInterval)

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
