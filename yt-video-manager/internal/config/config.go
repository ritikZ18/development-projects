package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	DownloadDir string `json:"download_dir"`
	Format      string `json:"format"` // best | 1080 | 720 | 480 | audio
}

func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ytm"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Default returns defaults. YTM_DOWNLOAD_DIR overrides the folder,
// which is how the Docker image points downloads at the mounted volume.
func Default() Config {
	dir := ""
	if d := os.Getenv("YTM_DOWNLOAD_DIR"); d != "" {
		dir = d
	} else {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "Downloads", "ytm")
	}
	return Config{DownloadDir: dir, Format: "best"}
}

// Load reads the config file, falling back to defaults if it doesn't exist.
func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return Config{}, err
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	if c.DownloadDir == "" {
		c.DownloadDir = Default().DownloadDir
	}
	if c.Format == "" {
		c.Format = "best"
	}
	return c, nil
}

// Save writes the config to disk, creating the directory if needed.
func (c Config) Save() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0o644)
}