package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"httpBackupGo/backup"
	"httpBackupGo/config"
	"httpBackupGo/web"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfgPath := defaultConfigPath()
	events := make(chan web.Event, 8)

	// Load initial config ONCE (also creates it if missing)
	cfg, err := config.LoadOrCreate(cfgPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	log.Printf("config loaded from %s", cfgPath)

	// Start Web UI (port/address comes from config; changes require restart)
	go func() {
		if err := web.StartServer(cfgPath, cfg.WebListenAddr, events); err != nil {
			log.Fatalf("web server failed: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Signal handling
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	intervalMin := normalizeInterval(cfg.IntervalMinutes)
	ticker := time.NewTicker(time.Duration(intervalMin) * time.Minute)
	defer ticker.Stop()
	log.Printf("scheduler started (interval=%d minutes)", intervalMin)

	var running atomic.Bool // prevents overlapping runs

	triggerRun := func(reason string) {
		if !running.CompareAndSwap(false, true) {
			log.Printf("run skipped (%s): already running", reason)
			return
		}
		go func() {
			defer running.Store(false)

			cfgNow, err := config.LoadOrCreate(cfgPath)
			if err != nil {
				log.Printf("failed to reload config: %v", err)
				return
			}

			runOnce(ctx, cfgNow)
		}()
	}

	reloadTickerIfNeeded := func() {
		cfgNow, err := config.LoadOrCreate(cfgPath)
		if err != nil {
			log.Printf("failed to reload config: %v", err)
			return
		}

		newInterval := normalizeInterval(cfgNow.IntervalMinutes)
		if newInterval != intervalMin {
			intervalMin = newInterval
			ticker.Stop()
			ticker = time.NewTicker(time.Duration(intervalMin) * time.Minute)
			log.Printf("scheduler interval updated to %d minutes", intervalMin)
		}
	}

	for {
		select {
		case <-ticker.C:
			triggerRun("ticker")

		case ev := <-events:
			switch ev.Type {
			case web.EventConfigChanged:
				log.Println("event: config changed -> reloading scheduler")
				reloadTickerIfNeeded()

			case web.EventRunNow:
				log.Println("event: run now")
				triggerRun("run-now")
			}

		case <-sig:
			log.Println("shutdown signal received")
			cancel()
			return

		case <-ctx.Done():
			return
		}
	}
}

func runOnce(ctx context.Context, cfg config.Config) {
	maxPar := 5
	if v := os.Getenv("HTTPBACKUP_MAX_PARALLEL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxPar = n
		}
	}

	r := backup.NewRunner(maxPar)
	r.RunAllEnabled(ctx, cfg)
}

func normalizeInterval(v int) int {
	if v <= 0 {
		return 1
	}
	return v
}

func defaultConfigPath() string {
	if pd := os.Getenv("ProgramData"); pd != "" {
		return filepath.Join(pd, "httpBackupGo", "config.json")
	}
	return "config.json"
}
