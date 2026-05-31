package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"streamdeck-go/internal/deck"
	"streamdeck-go/internal/render"
)

func TestLoadJSONConfigAppliesSettingsAndPages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "streamdeck.json")
	data := []byte(`{
		"version": 1,
		"settings": {
			"device": "auto",
			"icon_dir": "/tmp/icons",
			"brightness": 30,
			"hold_ms": 1200,
			"font": { "path": "/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf" },
			"media": { "player": "vlc" },
			"weather": { "location": "Cordoba", "refresh_minutes": 5 },
			"start_page": "main"
		},
		"pages": {
			"main": {
				"timeout_seconds": 7,
				"background": { "type": "media_art", "player": "vlc", "mode": "fill" },
				"buttons": {
					"0": {
						"layers": [
							{
								"type": "color",
								"color": "#123456",
								"effect": {
									"type": "blink",
									"color": "#abcdef",
									"blink_ms": 250,
									"duration_ms": 1000,
									"repeat": 2
								}
							},
							{ "type": "media_play_pause", "player": "vlc" },
							{ "type": "text", "text": "Play", "font_size": 14, "position": "lower" }
						],
						"press": { "type": "media", "player": "vlc", "command": "play_pause" },
						"hold": { "type": "media", "player": "vlc", "command": "stop" }
					},
					"1": {
						"layers": [
							{ "type": "datetime", "format": "HH:mm" }
						],
						"press": { "type": "brightness", "command": "set", "value": 45, "step": 5 }
					}
				}
			}
		}
	}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadJSONConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.DevicePath != "auto" {
		t.Fatalf("DevicePath = %q, want auto", cfg.DevicePath)
	}
	if cfg.IconDir != "/tmp/icons" {
		t.Fatalf("IconDir = %q, want /tmp/icons", cfg.IconDir)
	}
	if cfg.FontPath != "/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf" {
		t.Fatalf("FontPath = %q, want LiberationSans-Regular.ttf", cfg.FontPath)
	}
	if cfg.Brightness != 30 {
		t.Fatalf("Brightness = %d, want 30", cfg.Brightness)
	}
	if cfg.HoldMS != 1200 {
		t.Fatalf("HoldMS = %d, want 1200", cfg.HoldMS)
	}
	if cfg.MediaPlayer != "vlc" {
		t.Fatalf("MediaPlayer = %q, want vlc", cfg.MediaPlayer)
	}
	if cfg.Location != "Cordoba" {
		t.Fatalf("Location = %q, want Cordoba", cfg.Location)
	}
	if cfg.WeatherRefresh != 5*time.Minute {
		t.Fatalf("WeatherRefresh = %s, want 5m", cfg.WeatherRefresh)
	}
	if got := cfg.Pages["main"].Buttons[0].Press.Player; got != "vlc" {
		t.Fatalf("button media player = %q, want vlc", got)
	}
	if got := cfg.Pages["main"].Buttons[0].Layers[0].Color; got != "#123456" {
		t.Fatalf("button color layer color = %q, want #123456", got)
	}
	if got := cfg.Pages["main"].Buttons[0].Layers[0].Effect.BlinkMS; got != 250 {
		t.Fatalf("button color layer effect blink_ms = %d, want 250", got)
	}
	if got := cfg.Pages["main"].Buttons[0].Hold.Command; got != "stop" {
		t.Fatalf("button hold command = %q, want stop", got)
	}
	if got := cfg.Pages["main"].Buttons[1].Press.Step; got != 5 {
		t.Fatalf("button brightness step = %d, want 5", got)
	}
	if got := cfg.Pages["main"].Buttons[1].Press.Value; got == nil || *got != 45 {
		t.Fatalf("button brightness value = %v, want 45", got)
	}
	if got := cfg.Pages["main"].Buttons[0].Layers[2].Text; got != "Play" {
		t.Fatalf("button layer text = %q, want Play", got)
	}
	if got := len(cfg.Pages["main"].Buttons); got != deck.MaxKeyCount {
		t.Fatalf("buttons length = %d, want %d", got, deck.MaxKeyCount)
	}
	if got := cfg.Pages["main"].TimeoutSeconds; got != 7 {
		t.Fatalf("page timeout = %d, want 7", got)
	}
}

func TestLoadJSONConfigRejectsButtonArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "streamdeck.json")
	data := []byte(`{
		"version": 1,
		"settings": {
			"device": "auto",
			"icon_dir": "/tmp/icons",
			"brightness": 30,
			"hold_ms": 1200,
			"media": { "player": "vlc" },
			"weather": { "location": "Cordoba", "refresh_minutes": 5 },
			"start_page": "main"
		},
		"pages": {
			"main": {
				"background": { "type": "solid" },
				"buttons": [
					{
						"layers": [
							{ "type": "datetime", "format": "HH:mm" }
						],
						"press": { "type": "page", "page": "main" }
					}
				]
			}
		}
	}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadJSONConfig(path); err == nil {
		t.Fatal("LoadJSONConfig succeeded with button array")
	}
}

func TestLoadJSONConfigRejectsOutOfRangeButtonNumber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "streamdeck.json")
	data := []byte(`{
		"version": 1,
		"settings": {
			"device": "auto",
			"icon_dir": "/tmp/icons",
			"brightness": 30,
			"hold_ms": 1200,
			"media": { "player": "vlc" },
			"weather": { "location": "Cordoba", "refresh_minutes": 5 },
			"start_page": "main"
		},
		"pages": {
			"main": {
				"background": { "type": "solid" },
				"buttons": {
					"32": {
						"layers": [
							{ "type": "empty" }
						],
						"press": { "type": "empty", "command": "" }
					}
				}
			}
		}
	}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadJSONConfig(path); err == nil {
		t.Fatal("LoadJSONConfig succeeded with out-of-range button number")
	}
}

func TestLoadJSONConfigRejectsUnknownModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "streamdeck.json")
	data := []byte(`{
		"version": 1,
		"settings": {
			"device": "auto",
			"model": "plus",
			"icon_dir": "/tmp/icons",
			"brightness": 30,
			"hold_ms": 1200,
			"media": { "player": "vlc" },
			"weather": { "location": "Cordoba", "refresh_minutes": 5 },
			"start_page": "main"
		},
		"pages": {
			"main": {
				"background": { "type": "solid" },
				"buttons": {}
			}
		}
	}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadJSONConfig(path); err == nil {
		t.Fatal("LoadJSONConfig succeeded with unknown model")
	}
}

func TestLoadJSONConfigRejectsMissingPages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "streamdeck.json")
	data := []byte(`{
		"version": 1,
		"settings": {
			"device": "auto",
			"icon_dir": "/tmp/icons",
			"brightness": 30,
			"hold_ms": 1200,
			"media": { "player": "vlc" },
			"weather": { "location": "Cordoba", "refresh_minutes": 5 },
			"start_page": "main"
		}
	}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadJSONConfig(path); err == nil {
		t.Fatal("LoadJSONConfig succeeded without pages")
	}
}

func TestPageHasAnimation(t *testing.T) {
	if pageHasAnimation(Page{}) {
		t.Fatal("empty page reported animation")
	}
	page := Page{
		Buttons: []Button{
			{Layers: []render.Layer{{Type: "text", Text: "Static"}}},
			{Layers: []render.Layer{{Type: "animation", Path: "blink.gif"}}},
		},
	}
	if !pageHasAnimation(page) {
		t.Fatal("page with animation layer was not detected")
	}
	effectPage := Page{
		Background: render.Background{
			Effect: render.Effect{Type: "blink"},
		},
	}
	if !pageHasAnimation(effectPage) {
		t.Fatal("page with background effect was not detected")
	}
	colorLayerEffectPage := Page{
		Buttons: []Button{
			{Layers: []render.Layer{{Type: "color", Color: "#111111", Effect: render.Effect{Type: "blink"}}}},
		},
	}
	if !pageHasAnimation(colorLayerEffectPage) {
		t.Fatal("page with color layer effect was not detected")
	}
}
