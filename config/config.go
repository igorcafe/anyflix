package config

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
)

type Addon struct {
	Name     string
	Manifest string
}

type Config struct {
	PlayerCmd   string
	DownloadDir string
	SubLangs    []string
	Addons      []Addon
}

func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		PlayerCmd:   "mpv {{.URL}} {{range .Subs}} --sub-file={{.URL}} {{end}}",
		DownloadDir: filepath.Join(home, "Downloads", "anyflix"),
		SubLangs:    []string{"pob"},
		Addons:      []Addon{},
	}
}

func Load() (Config, error) {
	var cfg Config

	path, err := os.UserConfigDir()
	if err != nil {
		return cfg, err
	}

	path = filepath.Join(path, "anyflix.json")

	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg = DefaultConfig()
		err = Save(cfg)
		slog.Debug("initialized with default config")
		return cfg, err
	}
	if err != nil {
		return cfg, err
	}

	err = json.Unmarshal(b, &cfg)
	if err != nil {
		return cfg, err
	}

	slog.Debug("loaded config", "path", path)
	return cfg, nil
}

func Save(cfg Config) error {
	path, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	path = filepath.Join(path, "anyflix.json")

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(path, b, 0666)
	if err != nil {
		return err
	}

	slog.Debug("saved config", "path", path)

	return nil
}
