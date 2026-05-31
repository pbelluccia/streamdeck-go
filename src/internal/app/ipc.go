package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxInjectedPageBytes = 1 << 20

func DefaultSocketPath() string {
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "streamdeck-go.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("streamdeck-go-%d.sock", os.Getuid()))
}

func (a *App) startIPC(ctx context.Context) error {
	socketPath := DefaultSocketPath()
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale IPC socket: %w", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen on IPC socket %s: %w", socketPath, err)
	}
	if err := os.Chmod(socketPath, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(socketPath)
		return fmt.Errorf("chmod IPC socket: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/pages/", a.handleInjectedPage)
	server := &http.Server{Handler: mux}

	go func() {
		<-ctx.Done()
		_ = server.Close()
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}()
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Println("IPC server error:", err)
		}
	}()

	fmt.Println("IPC socket:", socketPath)
	return nil
}

func (a *App) handleInjectedPage(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/pages/"), "/")
	parts := strings.Split(path, "/")
	if path == "" || len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] != "clear") {
		http.Error(w, "page id must be one path segment", http.StatusBadRequest)
		return
	}
	pageID := parts[0]

	if len(parts) == 2 {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method must be POST", http.StatusMethodNotAllowed)
			return
		}
		if err := a.ClearInjectedPage(r.Context(), pageID); err != nil {
			http.Error(w, fmt.Sprintf("clear page: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page":   pageID,
			"active": false,
		})
		return
	}

	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		w.Header().Set("Allow", "PUT, POST")
		http.Error(w, "method must be PUT or POST", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	var page Page
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxInjectedPageBytes))
	if err := decoder.Decode(&page); err != nil {
		http.Error(w, fmt.Sprintf("decode page: %v", err), http.StatusBadRequest)
		return
	}

	if err := a.InjectPage(r.Context(), pageID, page); err != nil {
		http.Error(w, fmt.Sprintf("inject page: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"page":   pageID,
		"active": true,
	})
}

func (a *App) InjectPage(ctx context.Context, pageID string, page Page) error {
	a.stateMu.Lock()
	if a.config.Pages == nil {
		a.config.Pages = map[string]Page{}
	}
	a.config.Pages[pageID] = page
	a.currentPage = pageID
	a.stateMu.Unlock()

	a.resetPageTimeout(ctx)
	if a.deck == nil {
		return nil
	}
	return a.Refresh(ctx)
}

func (a *App) ClearInjectedPage(ctx context.Context, pageID string) error {
	a.stateMu.Lock()
	if pageID != a.config.StartPage {
		delete(a.config.Pages, pageID)
	}
	if a.currentPage == pageID {
		a.currentPage = a.config.StartPage
	}
	a.stateMu.Unlock()

	a.resetPageTimeout(ctx)
	if a.deck == nil {
		return nil
	}
	return a.Refresh(ctx)
}
