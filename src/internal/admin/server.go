package admin

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Options struct {
	ConfigPath string
	Addr       string
}

type Server struct {
	configPath string
	addr       string
	http       *http.Server
}

func New(options Options) *Server {
	addr := strings.TrimSpace(options.Addr)
	if addr == "" {
		addr = "127.0.0.1:8787"
	}
	s := &Server{configPath: options.ConfigPath, addr: addr}
	mux := http.NewServeMux()
	s.routes(mux)
	s.http = &http.Server{Addr: addr, Handler: mux}
	return s
}

func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) ListenAndServe() error {
	return s.http.ListenAndServe()
}

func (s *Server) routes(mux *http.ServeMux) {
	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	mux.HandleFunc("/", s.handleIndex)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/preview", s.handlePreview)
	mux.HandleFunc("/api/icons", s.handleIcons)
	mux.HandleFunc("/api/icon", s.handleIcon)
	mux.HandleFunc("/api/backups", s.handleBackups)
	mux.HandleFunc("/api/restore", s.handleRestore)
	mux.HandleFunc("/api/restart", s.handleRestart)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFileFS(w, r, staticFiles, "static/index.html")
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		_, cfg, err := readConfigFile(s.configPath)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		writeJSON(w, cfg)
	case http.MethodPut:
		var cfg FileConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, err, http.StatusBadRequest)
			return
		}
		data, err := encodeConfig(cfg)
		if err != nil {
			writeError(w, err, http.StatusBadRequest)
			return
		}
		if _, err := createBackup(s.configPath); err != nil {
			writeError(w, fmt.Errorf("backup before save: %w", err), http.StatusInternalServerError)
			return
		}
		if err := writeConfigFile(s.configPath, data); err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		restarted, err := restartService()
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "restarted": restarted})
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req PreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	preview, err := renderPreview(req)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, preview)
}

func (s *Server) handleIcons(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dir, err := s.iconDir(r)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	icons := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".png") {
			icons = append(icons, entry.Name())
		}
	}
	sort.Strings(icons)
	writeJSON(w, icons)
}

func (s *Server) handleIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dir, err := s.iconDir(r)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" || filepath.Base(name) != name || !strings.EqualFold(filepath.Ext(name), ".png") {
		writeError(w, fmt.Errorf("invalid icon name"), http.StatusBadRequest)
		return
	}
	path := filepath.Join(dir, name)
	cleanDir, err := filepath.Abs(dir)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	if filepath.Dir(cleanPath) != cleanDir {
		writeError(w, fmt.Errorf("invalid icon path"), http.StatusBadRequest)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, cleanPath)
}

func (s *Server) iconDir(r *http.Request) (string, error) {
	dir := strings.TrimSpace(r.URL.Query().Get("dir"))
	if dir != "" {
		return dir, nil
	}
	_, cfg, err := readConfigFile(s.configPath)
	if err != nil {
		return "", err
	}
	return cfg.Settings.IconDir, nil
}

func (s *Server) handleBackups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		backups, err := listBackups(s.configPath)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		writeJSON(w, backups)
	case http.MethodPost:
		backup, err := createBackup(s.configPath)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		writeJSON(w, backup)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if err := restoreBackup(s.configPath, req.Name); err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	restarted, err := restartService()
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "restarted": restarted})
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	restarted, err := restartService()
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "restarted": restarted})
}

func restartService() (bool, error) {
	cmd := exec.Command("systemctl", "--user", "restart", "streamdeck-go.service")
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("restart streamdeck-go.service: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return true, nil
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
