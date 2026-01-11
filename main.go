package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"httpBackupGo/backup"
	"httpBackupGo/config"
	"httpBackupGo/web"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// --- Config path ---
	cfgPath := defaultConfigPath()

	cfg, err := config.LoadOrCreate(cfgPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	log.Printf("config loaded from %s", cfgPath)

	// --- Context for graceful shutdown ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- Start Web UI ---
	go func() {
		if err := web.StartServer(cfgPath); err != nil {
			log.Fatalf("web server failed: %v", err)
		}
	}()

	// --- Scheduler ---
	ticker := time.NewTicker(time.Duration(cfg.IntervalMinutes) * time.Minute)
	defer ticker.Stop()

	log.Printf("scheduler started (interval=%d minutes)", cfg.IntervalMinutes)

	// --- Signal handling ---
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			log.Println("scheduler tick")
			runOnce(ctx, cfgPath)

		case <-sig:
			log.Println("shutdown signal received")
			cancel()
			return

		case <-ctx.Done():
			return
		}
	}
}

func runOnce(ctx context.Context, cfgPath string) {
	cfg,
		err := config.LoadOrCreate(cfgPath)
	if err != nil {
		log.Printf("failed to reload config: %v", err)
		return
	}

	maxPar := 5
	if v := os.Getenv("HTTPBACKUP_MAX_PARALLEL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxPar = n
		}
	}

	r := backup.NewRunner(maxPar)
	r.RunAllEnabled(ctx, cfg)
}

func defaultConfigPath() string {
	// Windows: %ProgramData%\httpBackupGo\config.json
	if pd := os.Getenv("ProgramData"); pd != "" {
		return filepath.Join(pd, "httpBackupGo", "config.json")
	}

	// Linux / fallback
	return "config.json"
}
