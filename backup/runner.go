package backup

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"httpBackupGo/config"
	"httpBackupGo/retention"
)

type Runner struct {
	HTTPClient  *http.Client
	MaxParallel int
}

// NewRunner creates a runner with sane defaults.
// MaxParallel is used to limit concurrent downloads.
func NewRunner(maxParallel int) *Runner {
	if maxParallel <= 0 {
		maxParallel = 5
	}

	return &Runner{
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		MaxParallel: maxParallel,
	}
}

// RunAllEnabled runs backups for all enabled sites.
// Each enabled site downloads concurrently, limited by MaxParallel.
func (r *Runner) RunAllEnabled(ctx context.Context, cfg config.Config) {
	sites := make([]config.Site, 0, len(cfg.Sites))
	for _, s := range cfg.Sites {
		if s.Enabled {
			sites = append(sites, s)
		}
	}

	if len(sites) == 0 {
		slog.Info("backup: no enabled sites")
		return
	}

	sem := make(chan struct{}, r.MaxParallel)
	var wg sync.WaitGroup

	slog.Info(
		"backup: starting run",
		"enabled_sites", len(sites),
		"max_parallel", r.MaxParallel,
	)

	for _, site := range sites {
		site := site // capture
		wg.Add(1)

		go func() {
			defer wg.Done()

			// Concurrency limiter
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				slog.Warn("backup: run aborted by context", "site", site.Name)
				return
			}

			if err := r.RunOneSite(ctx, cfg, site); err != nil {
				slog.Error(
					"backup: site failed",
					"site", site.Name,
					"url", site.Url,
					"err", err,
				)
			} else {
				slog.Info(
					"backup: site ok",
					"site", site.Name,
					"url", site.Url,
				)
			}
		}()
	}

	wg.Wait()
	slog.Info("backup: run finished")
}

// RunOneSite performs the actual download and saves it to:
//
//	<BackupFolder>/<Name>/backup_<Name>_DD-MM-YYYY_HH-mm-ss.zip
func (r *Runner) RunOneSite(ctx context.Context, cfg config.Config, site config.Site) error {
	start := time.Now()

	name := strings.TrimSpace(site.Name)
	if name == "" {
		return fmt.Errorf("site name is empty")
	}
	url := strings.TrimSpace(site.Url)
	if url == "" {
		return fmt.Errorf("site url is empty")
	}

	base := filepath.Clean(cfg.BackupFolder)
	siteDir := filepath.Join(base, name)

	// Ensure folder exists
	if err := os.MkdirAll(siteDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", siteDir, err)
	}

	ts := time.Now().Format("02-01-2006_15-04-05")
	filename := fmt.Sprintf("backup_%s_%s.zip", name, ts)
	outPath := filepath.Join(siteDir, filename)

	// Create temp file first, then rename (atomic-ish)
	tmpPath := outPath + ".tmp"

	slog.Info(
		"backup: download started",
		"site", name,
		"url", url,
		"out_path", outPath,
	)

	// Build request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("User-Agent", "httpBackupGo/1.0")

	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read a tiny snippet for debugging (don’t blow memory)
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("http status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create %q: %w", tmpPath, err)
	}
	defer func() { _ = f.Close() }()

	// Stream copy
	written, err := io.Copy(f, resp.Body)
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write file: %w", err)
	}

	// Ensure data flushed
	if err := f.Sync(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sync file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close file: %w", err)
	}

	// Replace tmp with final
	if err := os.Rename(tmpPath, outPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename to final: %w", err)
	}

	slog.Info(
		"backup: saved",
		"site", name,
		"url", url,
		"path", outPath,
		"bytes", written,
		"status_code", resp.StatusCode,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	// Apply retention (best-effort; never fail the backup)
	if err := retention.CleanupSite(siteDir, name, cfg.Retention); err != nil {
		slog.Warn(
			"retention: cleanup error",
			"site", name,
			"site_dir", siteDir,
			"retention", cfg.Retention,
			"err", err,
		)
	}

	return nil
}
