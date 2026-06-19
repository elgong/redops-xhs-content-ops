package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	cfg := LoadConfig()
	ctx := context.Background()

	store, err := openStore(ctx, cfg)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer store.Close()

	if cfg.SeedData {
		if err := store.Seed(ctx); err != nil {
			log.Fatalf("seed data: %v", err)
		}
	}

	service := NewAppService(store, NewConfiguredXHSAdapter(cfg), NewConfiguredContentAI(cfg))
	if cfg.SchedulerEnabled {
		go startScheduler(context.Background(), service)
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           NewServer(store, service, cfg).routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("RED OPS started at http://127.0.0.1%s", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func openStore(ctx context.Context, cfg Config) (Store, error) {
	if StoreMode() != "mysql" {
		log.Println("store mode: memory")
		return NewMemoryStore(), nil
	}
	log.Println("store mode: mysql")
	db, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if cfg.AutoMigrate {
		if err := Migrate(ctx, db); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	return NewMySQLStore(db), nil
}

func startScheduler(ctx context.Context, service *AppService) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := service.RunDuePublishes(ctx, time.Now())
			if err != nil {
				log.Printf("scheduler: %v", err)
				continue
			}
			if count > 0 {
				log.Printf("scheduler published %d task(s)", count)
			}
		}
	}
}
