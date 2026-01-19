package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"httpBackupGo/backup"
	"httpBackupGo/config"
	"httpBackupGo/logging"
	"httpBackupGo/web"
)

func main() {
	// ---- Logging (JSON) ----
	logPath := defaultLogPath()

	logger, closeLogs, err := logging.New(logging.Options{
		FilePath: logPath,
		ToStdout: true, // journald-friendly
		Level:    slog.LevelInfo,
	})
	if err != nil {
		panic(err)
	}
	defer closeLogs()

	slog.SetDefault(logger)
	slog.Info("logging initialized", "log_path", logPath)

	// Optional legacy logger flags (can remove once all log.* calls are gone)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// ---- Config ----
	cfgPath := defaultConfigPath()
	events := make(chan web.Event, 8) // buffered so UI never blocks

	// Load initial config ONCE (also creates it if missing)
	cfg, err := config.LoadOrCreate(cfgPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	slog.Info("config loaded", "path", cfgPath)

	// ---- Start Web UI (addr from config; changes require restart) ----
	go func(addr string) {
		if err := web.StartServer(cfgPath, addr, events); err != nil {
			log.Fatalf("web server failed: %v", err)
		}
	}(cfg.WebListenAddr)

	// ---- Context + signal handling ----
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	// ---- Scheduler (IntervalMinutes==0 disables auto runs) ----
	intervalMin := normalizeInterval(cfg.IntervalMinutes) // 0 stays 0
	var ticker *time.Ticker
	var tickCh <-chan time.Time

	if intervalMin > 0 {
		ticker = time.NewTicker(time.Duration(intervalMin) * time.Minute)
		tickCh = ticker.C
		slog.Info("scheduler started", "interval_minutes", intervalMin)
	} else {
		slog.Info("scheduler disabled (IntervalMinutes=0)")
	}

	// Prevent overlapping runs
	var running atomic.Bool

	triggerRun := func(reason string) {
		if !running.CompareAndSwap(false, true) {
			slog.Warn("run skipped: already running", "reason", reason)
			return
		}

		go func() {
			defer running.Store(false)

			cfgNow, err := config.LoadOrCreate(cfgPath)
			if err != nil {
				slog.Error("failed to reload config", "err", err)
				return
			}

			runOnce(ctx, cfgNow)
		}()
	}

	reloadTickerIfNeeded := func() {
		cfgNow, err := config.LoadOrCreate(cfgPath)
		if err != nil {
			slog.Error("failed to reload config", "err", err)
			return
		}

		newInterval := normalizeInterval(cfgNow.IntervalMinutes)

		// disabled -> enabled
		if intervalMin == 0 && newInterval > 0 {
			ticker = time.NewTicker(time.Duration(newInterval) * time.Minute)
			tickCh = ticker.C
			intervalMin = newInterval
			slog.Info("scheduler enabled", "interval_minutes", intervalMin)
			return
		}

		// enabled -> disabled
		if intervalMin > 0 && newInterval == 0 {
			if ticker != nil {
				ticker.Stop()
			}
			ticker = nil
			tickCh = nil
			intervalMin = 0
			slog.Info("scheduler disabled (IntervalMinutes=0)")
			return
		}

		// enabled -> enabled (interval changed)
		if intervalMin > 0 && newInterval > 0 && newInterval != intervalMin {
			if ticker != nil {
				ticker.Stop()
			}
			ticker = time.NewTicker(time.Duration(newInterval) * time.Minute)
			tickCh = ticker.C
			intervalMin = newInterval
			slog.Info("scheduler interval updated", "interval_minutes", intervalMin)
		}
	}

	// ---- Main loop ----
	for {
		select {
		case <-tickCh:
			triggerRun("ticker")

		case ev := <-events:
			switch ev.Type {
			case web.EventConfigChanged:
				slog.Info("event: config changed -> reloading scheduler")
				reloadTickerIfNeeded()

			case web.EventRunNow:
				slog.Info("event: run now")
				triggerRun("run-now")
			}

		case <-sig:
			slog.Info("shutdown signal received")
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

// normalizeInterval keeps 0 as "disabled" and normalizes negative values.
func normalizeInterval(v int) int {
	if v < 0 {
		return 1
	}
	return v
}

func defaultConfigPath() string {
	// Windows: %ProgramData%\httpBackupGo\config.json
	if pd := os.Getenv("ProgramData"); pd != "" {
		return filepath.Join(pd, "httpBackupGo", "config.json")
	}

	// Linux / macOS: ./config.json
	return "config.json"
}

func defaultLogPath() string {
	// Windows: %ProgramData%\httpBackupGo\log.json
	if pd := os.Getenv("ProgramData"); pd != "" {
		return filepath.Join(pd, "httpBackupGo", "log.json")
	}

	// Linux / macOS: ./log.json
	return "log.json"
}
