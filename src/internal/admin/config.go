package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"streamdeck-go/internal/app"
)

type FileConfig struct {
	Version  int      `json:"version"`
	Settings Settings `json:"settings"`
	Pages    Pages    `json:"pages"`
}

type Settings struct {
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
}

type Pages struct {
	Order []string
	Items map[string]app.Page
}

type BackupInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

func readConfigFile(path string) ([]byte, FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, FileConfig{}, err
	}
	var cfg FileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, FileConfig{}, err
	}
	if _, err := app.DecodeJSONConfig(data); err != nil {
		return nil, FileConfig{}, err
	}
	return data, cfg, nil
}

func encodeConfig(cfg FileConfig) ([]byte, error) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if _, err := app.DecodeJSONConfig(data); err != nil {
		return nil, err
	}
	if err := validatePageActions(cfg); err != nil {
		return nil, err
	}
	return data, nil
}

func writeConfigFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func validatePageActions(cfg FileConfig) error {
	if len(cfg.Pages.Items) == 0 {
		return fmt.Errorf("pages must not be empty")
	}
	for pageID, page := range cfg.Pages.Items {
		for buttonIndex, button := range page.Buttons {
			if err := validateActionTarget(cfg.Pages, pageID, buttonIndex, "press", button.Press); err != nil {
				return err
			}
			if err := validateActionTarget(cfg.Pages, pageID, buttonIndex, "hold", button.Hold); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateActionTarget(pages Pages, pageID string, buttonIndex int, name string, action app.Action) error {
	if action.Type != "page" {
		return nil
	}
	target := strings.TrimSpace(action.Page)
	if target == "" {
		target = strings.TrimSpace(action.Command)
	}
	if target == "" {
		return fmt.Errorf("page %q button %d %s action has no target page", pageID, buttonIndex, name)
	}
	if _, ok := pages.Items[target]; !ok {
		return fmt.Errorf("page %q button %d %s action targets missing page %q", pageID, buttonIndex, name, target)
	}
	return nil
}

func (p *Pages) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("pages must be an object")
	}
	p.Order = nil
	p.Items = map[string]app.Page{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return fmt.Errorf("pages key must be a string")
		}
		var page app.Page
		if err := decoder.Decode(&page); err != nil {
			return err
		}
		p.Order = append(p.Order, key)
		p.Items[key] = page
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	return nil
}

func (p Pages) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	written := map[string]bool{}
	first := true
	writePair := func(key string, page app.Page) error {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		keyData, err := json.Marshal(key)
		if err != nil {
			return err
		}
		pageData, err := json.Marshal(page)
		if err != nil {
			return err
		}
		buf.Write(keyData)
		buf.WriteByte(':')
		buf.Write(pageData)
		written[key] = true
		return nil
	}
	for _, key := range p.Order {
		page, ok := p.Items[key]
		if !ok {
			continue
		}
		if err := writePair(key, page); err != nil {
			return nil, err
		}
	}
	extra := make([]string, 0, len(p.Items))
	for key := range p.Items {
		if !written[key] {
			extra = append(extra, key)
		}
	}
	sort.Strings(extra)
	for _, key := range extra {
		if err := writePair(key, p.Items[key]); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func (cfg *FileConfig) UnmarshalJSON(data []byte) error {
	type fileConfigAlias FileConfig
	var alias fileConfigAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*cfg = FileConfig(alias)
	if cfg.Pages.Items == nil {
		cfg.Pages.Items = map[string]app.Page{}
	}
	return nil
}

func (cfg FileConfig) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{\n  \"version\": ")
	version, err := json.Marshal(cfg.Version)
	if err != nil {
		return nil, err
	}
	buf.Write(version)
	buf.WriteString(",\n  \"settings\": ")
	settings, err := json.MarshalIndent(cfg.Settings, "  ", "  ")
	if err != nil {
		return nil, err
	}
	buf.Write(settings)
	buf.WriteString(",\n  \"pages\": ")
	pages, err := cfg.Pages.MarshalJSON()
	if err != nil {
		return nil, err
	}
	if err := indentJSON(&buf, pages, "  "); err != nil {
		return nil, err
	}
	buf.WriteString("\n}")
	return buf.Bytes(), nil
}

func indentJSON(dst *bytes.Buffer, data []byte, prefix string) error {
	var indented bytes.Buffer
	if err := json.Indent(&indented, data, prefix, "  "); err != nil {
		return err
	}
	_, err := io.Copy(dst, &indented)
	return err
}

func backupDir(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "backups")
}

func createBackup(configPath string) (BackupInfo, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return BackupInfo{}, err
	}
	dir := backupDir(configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return BackupInfo{}, err
	}
	name := "config-" + time.Now().Format("20060102-150405") + ".json"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return BackupInfo{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return BackupInfo{}, err
	}
	return backupInfo(name, info), nil
}

func listBackups(configPath string) ([]BackupInfo, error) {
	entries, err := os.ReadDir(backupDir(configPath))
	if os.IsNotExist(err) {
		return []BackupInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	backups := make([]BackupInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		backups = append(backups, backupInfo(entry.Name(), info))
	}
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Name > backups[j].Name
	})
	return backups, nil
}

func restoreBackup(configPath, name string) error {
	if filepath.Base(name) != name || !strings.HasSuffix(name, ".json") {
		return fmt.Errorf("invalid backup name")
	}
	data, err := os.ReadFile(filepath.Join(backupDir(configPath), name))
	if err != nil {
		return err
	}
	var cfg FileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	if _, err := app.DecodeJSONConfig(data); err != nil {
		return err
	}
	if err := validatePageActions(cfg); err != nil {
		return err
	}
	if _, err := createBackup(configPath); err != nil {
		return err
	}
	return writeConfigFile(configPath, data)
}

func backupInfo(name string, info os.FileInfo) BackupInfo {
	return BackupInfo{
		Name:    name,
		Size:    info.Size(),
		ModTime: info.ModTime().Format(time.RFC3339),
	}
}
