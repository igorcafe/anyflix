package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Addon struct {
	Name     string
	Manifest string
}

func (a Addon) BaseURL() string {
	return strings.TrimSuffix(a.Manifest, "/"+path.Base(a.Manifest))
}

type Config struct {
	PlayerCmd string
	SubLangs  []string
	Addons    []Addon
}

func DefaultConfig() Config {
	return Config{
		PlayerCmd: "mpv {{.URL}} {{range .Subs}} --sub-file={{.URL}} {{end}}",
		SubLangs:  []string{"pob"},
		Addons:    []Addon{},
	}
}

func ConfigLoad() (Config, error) {
	var cfg Config

	path, err := os.UserConfigDir()
	if err != nil {
		return cfg, err
	}

	path = filepath.Join(path, "anyflix.json")

	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg = DefaultConfig()
		err = ConfigSave(cfg)
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

func ConfigSave(cfg Config) error {
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
