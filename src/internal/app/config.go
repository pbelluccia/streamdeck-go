package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"streamdeck-go/internal/deck"
	"streamdeck-go/internal/render"
)

type Page struct {
	TimeoutSeconds     int               `json:"timeout_seconds,omitempty"`
	IconTimeoutSeconds int               `json:"icon_timeout_seconds,omitempty"`
	Background         render.Background `json:"background"`
	Buttons            Buttons           `json:"buttons"`
}

type Buttons []Button

type Button struct {
	Layers []render.Layer `json:"layers"`
	Press  Action         `json:"press"`
	Hold   Action         `json:"hold,omitempty"`
}

type Action struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Player  string `json:"player,omitempty"`
	Page    string `json:"page,omitempty"`
	Step    int    `json:"step,omitempty"`
	Value   *int   `json:"value,omitempty"`
}

func (b *Buttons) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*b = nil
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		return fmt.Errorf("buttons must be a numbered object, not an array")
	}

	var keyed map[string]Button
	if err := json.Unmarshal(data, &keyed); err != nil {
		return err
	}
	buttons := make([]Button, deck.MaxKeyCount)
	for key, button := range keyed {
		index, err := strconv.Atoi(strings.TrimSpace(key))
		if err != nil {
			return fmt.Errorf("buttons key %q must be a number", key)
		}
		if index < 0 || index >= deck.MaxKeyCount {
			return fmt.Errorf("buttons key %d out of range 0-%d", index, deck.MaxKeyCount-1)
		}
		buttons[index] = button
	}
	*b = buttons
	return nil
}

func (b Buttons) MarshalJSON() ([]byte, error) {
	keyed := map[string]Button{}
	for index, button := range b {
		if isZeroButton(button) {
			continue
		}
		keyed[strconv.Itoa(index)] = button
	}
	return json.Marshal(keyed)
}

func isZeroButton(button Button) bool {
	return len(button.Layers) == 0 &&
		button.Press == (Action{}) &&
		button.Hold == (Action{})
}

type jsonConfig struct {
	Version  int `json:"version"`
	Settings struct {
		Device     string `json:"device"`
		Model      string `json:"model,omitempty"`
		IconDir    string `json:"icon_dir"`
		Brightness *int   `json:"brightness"`
		HoldMS     int    `json:"hold_ms"`
		Font       struct {
			Path string `json:"path"`
		} `json:"font"`
		Media struct {
			Player string `json:"player"`
		} `json:"media"`
		Weather struct {
			Location       string `json:"location"`
			RefreshMinutes int    `json:"refresh_minutes"`
		} `json:"weather"`
		StartPage string `json:"start_page"`
	} `json:"settings"`
	Pages map[string]Page `json:"pages"`
}

func DefaultConfigPath() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configHome = filepath.Join(home, ".config")
	}
	path := filepath.Join(configHome, "streamdeck-go")
	if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
		return path, nil
	}
	return filepath.Join(path, "config.json"), nil
}

func LoadConfig() (Config, error) {
	path, err := DefaultConfigPath()
	if err != nil {
		return Config{}, err
	}
	cfg, err := LoadJSONConfig(path)
	if err != nil {
		return Config{}, fmt.Errorf("load %s: %w", path, err)
	}
	return cfg, nil
}

func LoadJSONConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	return DecodeJSONConfig(data)
}

func DecodeJSONConfig(data []byte) (Config, error) {
	var file jsonConfig
	if err := json.Unmarshal(data, &file); err != nil {
		return Config{}, err
	}
	if file.Version != 1 {
		return Config{}, fmt.Errorf("unsupported config version %d", file.Version)
	}

	cfg := Config{
		DevicePath:     strings.TrimSpace(file.Settings.Device),
		DeviceModel:    deck.NormalizeModelID(file.Settings.Model),
		IconDir:        strings.TrimSpace(file.Settings.IconDir),
		FontPath:       strings.TrimSpace(file.Settings.Font.Path),
		Location:       strings.TrimSpace(file.Settings.Weather.Location),
		MediaPlayer:    strings.TrimSpace(file.Settings.Media.Player),
		HoldMS:         file.Settings.HoldMS,
		StartPage:      strings.TrimSpace(file.Settings.StartPage),
		Pages:          file.Pages,
		WeatherRefresh: time.Duration(file.Settings.Weather.RefreshMinutes) * time.Minute,
	}
	if file.Settings.Brightness != nil {
		cfg.Brightness = byte(*file.Settings.Brightness)
	}
	return validateConfig(cfg, file.Settings.Brightness)
}

func validateConfig(cfg Config, brightness *int) (Config, error) {
	if cfg.DevicePath == "" {
		return Config{}, fmt.Errorf("settings.device is required")
	}
	if err := deck.ValidateModelID(cfg.DeviceModel); err != nil {
		return Config{}, err
	}
	if cfg.IconDir == "" {
		return Config{}, fmt.Errorf("settings.icon_dir is required")
	}
	if brightness == nil {
		return Config{}, fmt.Errorf("settings.brightness is required")
	}
	if *brightness < 0 || *brightness > 100 {
		return Config{}, fmt.Errorf("settings.brightness must be between 0 and 100")
	}
	if cfg.HoldMS <= 0 {
		return Config{}, fmt.Errorf("settings.hold_ms must be greater than 0")
	}
	if cfg.MediaPlayer == "" {
		return Config{}, fmt.Errorf("settings.media.player is required")
	}
	if cfg.Location == "" {
		return Config{}, fmt.Errorf("settings.weather.location is required")
	}
	if cfg.WeatherRefresh <= 0 {
		return Config{}, fmt.Errorf("settings.weather.refresh_minutes must be greater than 0")
	}
	if cfg.StartPage == "" {
		return Config{}, fmt.Errorf("settings.start_page is required")
	}
	if len(cfg.Pages) == 0 {
		return Config{}, fmt.Errorf("pages must contain at least one page")
	}
	if _, ok := cfg.Pages[cfg.StartPage]; !ok {
		return Config{}, fmt.Errorf("settings.start_page %q does not exist in pages", cfg.StartPage)
	}
	return cfg, nil
}
