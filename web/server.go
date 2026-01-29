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
	events  chan<- Event
}

type viewModel struct {
	ConfigPath string
	Config     config.Config

	Message string
	Error   string
	Now     string
}

func StartServer(cfgPath string, addr string, events chan<- Event) error {
	s := &Server{cfgPath: cfgPath, events: events}

	// Parse ALL templates (index.html + admin.html, etc.)
	tpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}
	s.tpl = tpl

	mux := http.NewServeMux()

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("static fs sub: %w", err)
	}

	mux.Handle("/static/",
		http.StripPrefix("/static/",
			http.FileServer(http.FS(staticSub)),
		),
	)

	// Pages
	mux.HandleFunc("/", s.handleHome)       // NEW simple page
	mux.HandleFunc("/admin", s.handleAdmin) // OLD index moved here

	// Actions (keep as-is)
	mux.HandleFunc("/save", s.handleSave)
	mux.HandleFunc("/run", s.handleRun)
	mux.HandleFunc("/reload", s.handleReload)

	log.Printf("web ui listening on http://%s", addr)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return srv.ListenAndServe()
}

// NEW: simple landing page with Run button + link to /admin
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
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
	if err := s.tpl.ExecuteTemplate(w, "index.html", vm); err != nil {
		log.Printf("template execute error (home): %v", err)
	}
}

// NEW: admin page (your previous index UI)
func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
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
	if err := s.tpl.ExecuteTemplate(w, "admin.html", vm); err != nil {
		log.Printf("template execute error (admin): %v", err)
	}
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?err="+q("invalid form: "+err.Error()), http.StatusSeeOther)
		return
	}

	cfg, err := config.LoadOrCreate(s.cfgPath)
	if err != nil {
		http.Redirect(w, r, "/admin?err="+q("failed to load config: "+err.Error()), http.StatusSeeOther)
		return
	}
	webAddr := strings.TrimSpace(r.FormValue("WebListenAddr"))
	if webAddr != "" {
		cfg.WebListenAddr = webAddr
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
		http.Redirect(w, r, "/admin?err="+q("failed to save config: "+err.Error()), http.StatusSeeOther)
		return
	}

	// 🔔 Notify main immediately
	nonBlockingSend(s.events, Event{Type: EventConfigChanged})

	http.Redirect(w, r, "/admin?msg="+q("Config saved + scheduler reloaded"), http.StatusSeeOther)
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 🔔 Notify main to run immediately
	nonBlockingSend(s.events, Event{Type: EventRunNow})

	// If request came from homepage, send back to "/"
	if r.URL.Query().Get("from") == "home" {
		http.Redirect(w, r, "/?msg="+q("Run started"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin?msg="+q("Run started"), http.StatusSeeOther)
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 🔔 Force reload scheduler/config (useful button)
	nonBlockingSend(s.events, Event{Type: EventConfigChanged})

	http.Redirect(w, r, "/admin?msg="+q("Scheduler reloaded"), http.StatusSeeOther)
}

// nonBlockingSend prevents the web request from hanging if main is busy.
// If the channel buffer is full, we just drop the event (safe).
func nonBlockingSend(ch chan<- Event, ev Event) {
	if ch == nil {
		return
	}
	select {
	case ch <- ev:
	default:
	}
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
