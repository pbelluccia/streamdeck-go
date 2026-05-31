package admin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"streamdeck-go/internal/app"
	"streamdeck-go/internal/render"
)

func TestEncodeConfigKeepsNumberedButtonsAndValidatesPageTargets(t *testing.T) {
	brightness := 20
	cfg := FileConfig{
		Version: 1,
		Settings: Settings{
			Device:     "auto",
			IconDir:    "/tmp/icons",
			Brightness: &brightness,
			HoldMS:     1000,
			StartPage:  "main",
		},
		Pages: Pages{
			Order: []string{"main", "settings"},
			Items: map[string]app.Page{
				"main": {
					Background: render.Background{Type: "solid", Color: "#111827"},
					Buttons: app.Buttons{
						{
							Layers: []render.Layer{{Type: "text", Text: "Next"}},
							Press:  app.Action{Type: "page", Page: "settings"},
						},
					},
				},
				"settings": {
					Background: render.Background{Type: "solid", Color: "#000000"},
				},
			},
		},
	}
	cfg.Settings.Media.Player = "spotify"
	cfg.Settings.Weather.Location = "Buenos Aires"
	cfg.Settings.Weather.RefreshMinutes = 10

	data, err := encodeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	pages := raw["pages"].(map[string]any)
	main := pages["main"].(map[string]any)
	if _, ok := main["buttons"].(map[string]any); !ok {
		t.Fatalf("buttons encoded as %T, want object", main["buttons"])
	}
}

func TestConfigPreservesPageOrder(t *testing.T) {
	data := []byte(`{
  "version": 1,
  "settings": {
    "device": "auto",
    "icon_dir": "/tmp/icons",
    "brightness": 20,
    "hold_ms": 1000,
    "font": { "path": "" },
    "media": { "player": "spotify" },
    "weather": { "location": "Buenos Aires", "refresh_minutes": 10 },
    "start_page": "main"
  },
  "pages": {
    "main": { "background": { "type": "solid" }, "buttons": {} },
    "access": { "background": { "type": "solid" }, "buttons": {} },
    "color_effects": { "background": { "type": "solid" }, "buttons": {} }
  }
}`)
	var cfg FileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	want := []string{"main", "access", "color_effects"}
	if len(cfg.Pages.Order) != len(want) {
		t.Fatalf("order length = %d, want %d", len(cfg.Pages.Order), len(want))
	}
	for i := range want {
		if cfg.Pages.Order[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q", i, cfg.Pages.Order[i], want[i])
		}
	}
	encoded, err := encodeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	mainAt := strings.Index(text, `"main"`)
	accessAt := strings.Index(text, `"access"`)
	colorAt := strings.Index(text, `"color_effects"`)
	if !(mainAt >= 0 && mainAt < accessAt && accessAt < colorAt) {
		t.Fatalf("encoded page order changed:\n%s", text)
	}
}

func TestEncodeConfigRejectsMissingPageTarget(t *testing.T) {
	brightness := 20
	cfg := FileConfig{
		Version: 1,
		Settings: Settings{
			Device:     "auto",
			IconDir:    "/tmp/icons",
			Brightness: &brightness,
			HoldMS:     1000,
			StartPage:  "main",
		},
		Pages: Pages{
			Order: []string{"main"},
			Items: map[string]app.Page{
				"main": {
					Buttons: app.Buttons{
						{Press: app.Action{Type: "page", Page: "missing"}},
					},
				},
			},
		},
	}
	cfg.Settings.Media.Player = "spotify"
	cfg.Settings.Weather.Location = "Buenos Aires"
	cfg.Settings.Weather.RefreshMinutes = 10

	if _, err := encodeConfig(cfg); err == nil {
		t.Fatal("encodeConfig succeeded with missing page target")
	}
}

func TestBackupAndRestoreConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	original := []byte(validConfigJSON("main"))
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}

	backup, err := createBackup(path)
	if err != nil {
		t.Fatal(err)
	}
	if backup.Name == "" {
		t.Fatal("backup name is empty")
	}

	if err := os.WriteFile(path, []byte(validConfigJSON("other")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := restoreBackup(path, backup.Name); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != string(original) {
		t.Fatalf("restored config mismatch\n%s", restored)
	}
}

func validConfigJSON(startPage string) string {
	return `{
  "version": 1,
  "settings": {
    "device": "auto",
    "icon_dir": "/tmp/icons",
    "brightness": 20,
    "hold_ms": 1000,
    "font": { "path": "" },
    "media": { "player": "spotify" },
    "weather": { "location": "Buenos Aires", "refresh_minutes": 10 },
    "start_page": "` + startPage + `"
  },
  "pages": {
    "main": { "background": { "type": "solid" }, "buttons": {} },
    "other": { "background": { "type": "solid" }, "buttons": {} }
  }
}
`
}
