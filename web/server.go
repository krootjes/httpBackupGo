package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"httpBackupGo/config"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

type Server struct {
	cfgPath string
	tpl     *template.Template
	runNow  atomic.Int32
}

type viewModel struct {
	ConfigPath string
	Config     config.Config

	Message string
	Error   string
	Now     string
}

func StartServer(cfgPath string) error {
	s := &Server{cfgPath: cfgPath}

	tpl, err := template.ParseFS(templatesFS, "templates/index.html")
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	s.tpl = tpl

	mux := http.NewServeMux()

	// ✅ Serve embedded static files correctly (sub-FS rooted at "static")
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("static fs sub: %w", err)
	}

	mux.Handle("/static/",
		http.StripPrefix("/static/",
			http.FileServer(http.FS(staticSub)),
		),
	)

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/save", s.handleSave)
	mux.HandleFunc("/run", s.handleRun)

	addr := "127.0.0.1:8123"
	log.Printf("web ui listening on http://%s", addr)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return srv.ListenAndServe()
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg, err := config.LoadOrCreate(s.cfgPath)
	if err != nil {
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	vm := viewModel{
		ConfigPath: s.cfgPath,
		Config:     cfg,
		Now:        time.Now().Format(time.RFC3339),
		Message:    r.URL.Query().Get("msg"),
		Error:      r.URL.Query().Get("err"),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tpl.Execute(w, vm); err != nil {
		log.Printf("template execute error: %v", err)
	}
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?err="+q("invalid form: "+err.Error()), http.StatusSeeOther)
		return
	}

	cfg, err := config.LoadOrCreate(s.cfgPath)
	if err != nil {
		http.Redirect(w, r, "/?err="+q("failed to load config: "+err.Error()), http.StatusSeeOther)
		return
	}

	cfg.IntervalMinutes = parseInt(r.FormValue("IntervalMinutes"), cfg.IntervalMinutes)
	cfg.Retention = parseInt(r.FormValue("Retention"), cfg.Retention)

	backupFolder := strings.TrimSpace(r.FormValue("BackupFolder"))
	if backupFolder != "" {
		cfg.BackupFolder = filepath.Clean(backupFolder)
	}

	presentTokens := r.Form["SiteEnabledPresent"]
	enabledTokens := r.Form["SiteEnabled"]
	names := r.Form["SiteName"]
	urls := r.Form["SiteUrl"]

	n := max(len(presentTokens), len(names), len(urls))

	enabledSet := map[string]struct{}{}
	for _, t := range enabledTokens {
		enabledSet[t] = struct{}{}
	}

	sites := make([]config.Site, 0, n)
	for i := 0; i < n; i++ {
		token := ""
		if i < len(presentTokens) {
			token = presentTokens[i]
		}

		name := ""
		if i < len(names) {
			name = strings.TrimSpace(names[i])
		}

		url := ""
		if i < len(urls) {
			url = strings.TrimSpace(urls[i])
		}

		if name == "" && url == "" {
			continue
		}

		_, enabled := enabledSet[token]

		sites = append(sites, config.Site{
			Enabled: enabled,
			Name:    name,
			Url:     url,
		})
	}

	cfg.Sites = sites
	cfg.ValidateAndNormalize()

	if err := config.Save(s.cfgPath, cfg); err != nil {
		http.Redirect(w, r, "/?err="+q("failed to save config: "+err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/?msg="+q("Config saved"), http.StatusSeeOther)
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.runNow.Store(1)
	http.Redirect(w, r, "/?msg="+q("Run requested"), http.StatusSeeOther)
}

func parseInt(s string, fallback int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}

func max(a, b, c int) int {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}

func q(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "&", "%26")
	s = strings.ReplaceAll(s, "?", "%3F")
	s = strings.ReplaceAll(s, "#", "%23")
	s = strings.ReplaceAll(s, " ", "+")
	return s
}
