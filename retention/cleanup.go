package retention

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CleanupSite keeps at most `keep` backup zip files in `siteDir`.
// It removes the oldest files first.
// Files are matched by prefix "backup_<siteName>_" and suffix ".zip".
func CleanupSite(siteDir string, siteName string, keep int) error {
	if keep <= 0 {
		return nil // nothing to keep == do nothing (safest)
	}

	entries, err := os.ReadDir(siteDir)
	if err != nil {
		return fmt.Errorf("readdir %q: %w", siteDir, err)
	}

	prefix := "backup_" + siteName + "_"

	type fileInfo struct {
		name string
		path string
		mod  time.Time
	}

	var files []fileInfo

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()

		// Strict match: our backups only
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".zip") {
			continue
		}

		info, err := e.Info()
		if err != nil {
			log.Printf("retention: stat failed %s: %v", name, err)
			continue
		}

		files = append(files, fileInfo{
			name: name,
			path: filepath.Join(siteDir, name),
			mod:  info.ModTime(),
		})
	}

	if len(files) <= keep {
		return nil // nothing to delete
	}

	// Oldest first
	sort.Slice(files, func(i, j int) bool {
		return files[i].mod.Before(files[j].mod)
	})

	toDelete := files[:len(files)-keep]

	for _, f := range toDelete {
		if err := os.Remove(f.path); err != nil {
			log.Printf("retention: failed to remove %s: %v", f.path, err)
		} else {
			log.Printf("retention: removed old backup %s", f.name)
		}
	}

	return nil
}
