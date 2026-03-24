package main

import (
	"log"
	"net/http"
	"time"

	"github.com/janexpl/CoursesListNext/api/internal/config"
	"github.com/janexpl/CoursesListNext/api/internal/db"
	dbsql "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/server"
)

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

	log.Printf("api listening on :%s", cfg.Port)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}
