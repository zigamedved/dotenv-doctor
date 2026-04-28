// Package config loads and saves dotenv-doctor's user config.
//
// The config lives at ~/.config/envs/config.toml (or $XDG_CONFIG_HOME/envs/config.toml).
// It is intentionally tiny: scan paths and a few toggles. Anything more ambitious
// belongs in command-line flags, not persistent config.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the persistent user config.
type Config struct {
	// ScanPaths are the directories to walk when discovering projects.
	// Tilde-prefixed entries are expanded at load time.
	ScanPaths []string `toml:"scan_paths"`

	// MaxDepth caps how deep we walk under each scan path. 0 means use default.
	MaxDepth int `toml:"max_depth"`

	// SkipDirs is appended to the built-in skip set (node_modules, .git, etc.).
	SkipDirs []string `toml:"skip_dirs"`
}

// Defaults returns a sensible default config for first-run.
func Defaults() Config {
	return Config{
		ScanPaths: []string{"~/code"},
		MaxDepth:  4,
		SkipDirs:  nil,
	}
}

// Path returns the absolute path to the config file. It does not require the
// file to exist.
func Path() (string, error) {
	if env := os.Getenv("DOTENV_DOCTOR_CONFIG"); env != "" {
		return env, nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "envs", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "envs", "config.toml"), nil
}

// Exists reports whether the config file exists on disk.
func Exists() bool {
	p, err := Path()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Load reads the config from disk. Returns os.ErrNotExist if no config file exists;
// callers should branch on that.
func Load() (Config, error) {
	p, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return Config{}, err
	}
	var c Config
	if _, err := toml.Decode(string(data), &c); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", p, err)
	}
	c.ScanPaths = expandPaths(c.ScanPaths)
	if c.MaxDepth <= 0 {
		c.MaxDepth = 4
	}
	return c, nil
}

// Save writes the config to disk, creating parent dirs as needed.
func Save(c Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", p, err)
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return nil
}

// IsNotExist is a small helper so callers don't have to import os just for this.
func IsNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

func expandPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	home, _ := os.UserHomeDir()
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "~/") && home != "" {
			p = filepath.Join(home, p[2:])
		} else if p == "~" && home != "" {
			p = home
		}
		abs, err := filepath.Abs(p)
		if err == nil {
			p = abs
		}
		out = append(out, p)
	}
	return out
}
