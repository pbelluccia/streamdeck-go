package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleIconServesPNGFromIconDir(t *testing.T) {
	dir := t.TempDir()
	iconDir := filepath.Join(dir, "icons")
	if err := os.Mkdir(iconDir, 0o755); err != nil {
		t.Fatal(err)
	}
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	if err := os.WriteFile(filepath.Join(iconDir, "play.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := writeServerTestConfig(t, dir, iconDir)

	server := New(Options{ConfigPath: configPath})
	req := httptest.NewRequest(http.MethodGet, "/api/icon?name=play.png", nil)
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.HasPrefix(rec.Body.String(), string(png)) {
		t.Fatalf("body = %q, want PNG bytes", rec.Body.String())
	}
}

func TestHandleIconRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	iconDir := filepath.Join(dir, "icons")
	if err := os.Mkdir(iconDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := writeServerTestConfig(t, dir, iconDir)

	server := New(Options{ConfigPath: configPath})
	req := httptest.NewRequest(http.MethodGet, "/api/icon?name=../secret.png", nil)
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func writeServerTestConfig(t *testing.T, dir, iconDir string) string {
	t.Helper()
	configPath := filepath.Join(dir, "config.json")
	iconDirJSON, err := json.Marshal(iconDir)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte(`{
  "version": 1,
  "settings": {
    "device": "auto",
    "icon_dir": ` + string(iconDirJSON) + `,
    "brightness": 20,
    "hold_ms": 1000,
    "font": { "path": "" },
    "media": { "player": "spotify" },
    "weather": { "location": "Buenos Aires", "refresh_minutes": 10 },
    "start_page": "main"
  },
  "pages": {
    "main": { "background": { "type": "solid" }, "buttons": {} }
  }
}`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return configPath
}
