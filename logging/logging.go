package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Options lets you control where logs go.
type Options struct {
	// If set, logs are also appended to this file (JSON Lines).
	FilePath string

	// If true, logs are written to stdout (recommended for journald).
	ToStdout bool

	// Minimum level: slog.LevelInfo, slog.LevelDebug, etc.
	Level slog.Level
}

func New(opts Options) (*slog.Logger, func(), error) {
	var writers []io.Writer
	closeFn := func() {}

	if opts.ToStdout {
		writers = append(writers, os.Stdout)
	}

	var f *os.File
	if opts.FilePath != "" {
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(opts.FilePath), 0o755); err != nil {
			return nil, closeFn, err
		}

		var err error
		f, err = os.OpenFile(opts.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, closeFn, err
		}
		writers = append(writers, f)

		closeFn = func() {
			_ = f.Close()
		}
	}

	// Fallback: if nothing chosen, at least log to stdout
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	mw := io.MultiWriter(writers...)

	h := slog.NewJSONHandler(mw, &slog.HandlerOptions{
		Level: opts.Level,
		// Optional: make time RFC3339 instead of epoch
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					return slog.String(slog.TimeKey, t.Format(time.RFC3339))
				}
			}
			return a
		},
	})

	return slog.New(h), closeFn, nil
}
