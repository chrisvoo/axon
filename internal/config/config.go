package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chrisvoo/axon/internal/paths"
	"gopkg.in/yaml.v3"
)

//go:embed default.yaml
var defaultConfigYAML []byte

//go:embed denylist.txt
var defaultDenylist []byte

// WriteDefaultDenylist writes the embedded denylist if the target file does not exist.
func WriteDefaultDenylist(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, defaultDenylist, 0o600)
}

// Config holds Axon runtime configuration.
type Config struct {
	ListenAddr    string   `yaml:"listen_addr"`
	ListenPort    int      `yaml:"listen_port"`
	IPAllowlist   []string `yaml:"ip_allowlist"`
	RateLimitRPS  float64  `yaml:"rate_limit_rps"`
	IdleTimeout   string   `yaml:"idle_timeout"`
	ShellTimeout  string   `yaml:"shell_timeout"`
	InputStallSec int      `yaml:"input_stall_seconds"`
	ReadOnly      bool     `yaml:"read_only"`
	AuditLog      string   `yaml:"audit_log"`
	DenylistFile  string   `yaml:"denylist_file"`
	CertFile      string   `yaml:"cert_file"`
	KeyFile       string   `yaml:"key_file"`
	APIKey        string   `yaml:"api_key"`

	configDir string
}

// ConfigPath returns the path to the user config file.
func ConfigPath() (string, error) {
	dir, err := paths.EnsureConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads and merges default + user config from ~/.axon/config.yaml.
func Load() (*Config, error) {
	dir, err := paths.EnsureConfigDir()
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(defaultConfigYAML, &cfg); err != nil {
		return nil, fmt.Errorf("parse embedded default: %w", err)
	}

	path := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}

	cfg.configDir = dir
	if cfg.CertFile == "" {
		cfg.CertFile = filepath.Join(dir, "server.crt")
	}
	if cfg.KeyFile == "" {
		cfg.KeyFile = filepath.Join(dir, "server.key")
	}
	if cfg.AuditLog != "" && !filepath.IsAbs(cfg.AuditLog) {
		cfg.AuditLog = filepath.Join(dir, cfg.AuditLog)
	}
	if cfg.DenylistFile != "" && !filepath.IsAbs(cfg.DenylistFile) {
		cfg.DenylistFile = filepath.Join(dir, cfg.DenylistFile)
	}
	return &cfg, nil
}

// Save writes the config to disk (used after init or keygen).
func (c *Config) Save() error {
	dir, err := paths.EnsureConfigDir()
	if err != nil {
		return err
	}
	c.configDir = dir
	path := filepath.Join(dir, "config.yaml")
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Dir returns the config directory.
func (c *Config) Dir() string {
	return c.configDir
}

// ParseDuration parses cfg.ShellTimeout or returns default.
func (c *Config) ShellTimeoutDuration() time.Duration {
	d, err := time.ParseDuration(c.ShellTimeout)
	if err != nil || d <= 0 {
		return 5 * time.Minute
	}
	return d
}

// IdleTimeoutDuration parses idle timeout; 0 if disabled or invalid.
func (c *Config) IdleTimeoutDuration() time.Duration {
	if c.IdleTimeout == "" || c.IdleTimeout == "0" {
		return 0
	}
	d, err := time.ParseDuration(c.IdleTimeout)
	if err != nil {
		return 0
	}
	return d
}

// DenylistPath returns absolute path to denylist file.
func (c *Config) DenylistPath() string {
	return c.DenylistFile
}
