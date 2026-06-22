package config

import (
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/joho/godotenv/autoload"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

//go:embed config.yaml
var embeddedConfig []byte

type Config struct {
	Scheme struct {
		Address  string `mapstructure:"address"`
		HttpPort int    `mapstructure:"http_port"`
	} `mapstructure:"scheme"`

	BaseURL           string `mapstructure:"base_url"`
	PoolSize          int    `mapstructure:"pool_size"`
	LogLevel          string `mapstructure:"log_level"`
	SecretKey         string `mapstructure:"secret_key"`
	AppDebug          bool   `mapstructure:"app_debug"`
	DataPath          string `mapstructure:"data_path"`
	MaxmindLicenseKey string `mapstructure:"maxmind_license_key"`
	GeoIPMirror       string `mapstructure:"geoip_mirror"`

	SMTP struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
		From     string `mapstructure:"from"`
	} `mapstructure:"smtp"`

	Clickhouse struct {
		Addr     []string `mapstructure:"addr"`
		Database string   `mapstructure:"database"`
		Username string   `mapstructure:"username"`
		Password string   `mapstructure:"password"`
		Ssl      bool     `mapstructure:"ssl"`
	} `mapstructure:"clickhouse"`

	Database struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		Database string `mapstructure:"database"`
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
	} `mapstructure:"db"`
}

var (
	// Conf holds the current configuration. It is initialized with safe zero-value
	// defaults and replaced by Load() with the real environment-specific config.
	Conf = &Config{
		DataPath: "./data",
		LogLevel: "info",
	}
	mu sync.RWMutex
)

// Load reads configuration for the given environment.
// env must be "dev", "test", or "prod" (defaults to "dev" if empty).
func Load(env string) error {
	if env == "" {
		env = "dev"
	}
	slog.Info("config: loading", "env", env)

	// 1. Parse config.yaml and resolve YAML anchors / merge keys.
	envMap, err := resolveEnv(env)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// 2. Feed resolved map to Viper for env-var overrides.
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.MergeConfigMap(envMap); err != nil {
		return fmt.Errorf("config: merge map: %w", err)
	}

	// 3. Environment variable overrides (ZENSTATS_DB_HOST → db.host).
	v.SetEnvPrefix("ZENSTATS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindAllPaths(v)

	// 4. Optional external override file (./config/config_<env>.yaml).
	v.SetConfigName("config_" + env)
	v.AddConfigPath("./config")
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Warn("config: external file load failed", "env", env, "err", err)
		}
	}

	// 5. Unmarshal into struct.
	cfg := new(Config)
	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("config: unmarshal: %w", err)
	}

	// 6. Secret key is NOT in YAML — must come from environment.
	cfg.SecretKey = os.Getenv("ZENSTATS_SECRET_KEY")
	if cfg.SecretKey == "" {
		if env == "prod" {
			return fmt.Errorf("ZENSTATS_SECRET_KEY is required in production — set it via environment variable")
		}
		slog.Warn("config: ZENSTATS_SECRET_KEY is not set — JWT tokens will use an empty signing key. Set it via environment for production.")
	}

	// 7. ClickHouse addr: support comma-separated or JSON array env override.
	if addrStr := os.Getenv("ZENSTATS_CLICKHOUSE_ADDR"); addrStr != "" {
		if strings.HasPrefix(strings.TrimSpace(addrStr), "[") {
			yaml.Unmarshal([]byte(addrStr), &cfg.Clickhouse.Addr)
		} else {
			cfg.Clickhouse.Addr = strings.Split(addrStr, ",")
		}
	}

	// 8. Validate.
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config: validate: %w", err)
	}

	mu.Lock()
	Conf = cfg
	mu.Unlock()

	// Write resolved config to disk for transparency (best-effort).
	writeConfigFile(v, env)
	slog.Info("config: loaded", "env", env)

	return nil
}

// resolveEnv parses config.yaml and returns the fully-merged map for the given env.
// yaml.v3 natively resolves <<: merge keys and *anchors.
func resolveEnv(env string) (map[string]interface{}, error) {
	var root map[string]interface{}
	if err := yaml.Unmarshal(embeddedConfig, &root); err != nil {
		return nil, fmt.Errorf("parse config.yaml: %w", err)
	}

	envMap, ok := root[env].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("environment %q not found in config.yaml", env)
	}

	return envMap, nil
}

// bindAllPaths ensures all known config keys are discoverable by AutomaticEnv.
func bindAllPaths(v *viper.Viper) {
	paths := []string{
		"scheme.address", "scheme.http_port",
		"base_url", "pool_size", "log_level", "app_debug", "data_path",
		"maxmind_license_key", "geoip_mirror",
		"smtp.host", "smtp.port", "smtp.username", "smtp.password", "smtp.from",
		"db.host", "db.port", "db.database", "db.username", "db.password",
		"clickhouse.addr", "clickhouse.database", "clickhouse.username", "clickhouse.password", "clickhouse.ssl",
	}
	for _, p := range paths {
		_ = v.BindEnv(p)
	}
}

// writeConfigFile writes the effective config to ./config/config_<env>.yaml (best-effort).
func writeConfigFile(v *viper.Viper, env string) {
	configFile := filepath.Join("config", fmt.Sprintf("config_%s.yaml", env))
	if _, err := os.Stat(configFile); err == nil {
		return // don't overwrite existing user customizations
	}
	if err := os.MkdirAll(filepath.Dir(configFile), 0766); err != nil {
		return
	}
	f, err := os.Create(configFile)
	if err != nil {
		return
	}
	defer f.Close()
	_ = v.WriteConfig()
}

// Validate checks required fields and value ranges.
func (c *Config) Validate() error {
	if c.Scheme.HttpPort < 1 || c.Scheme.HttpPort > 65535 {
		return fmt.Errorf("scheme.http_port out of range: %d", c.Scheme.HttpPort)
	}
	if c.Database.Host == "" {
		return fmt.Errorf("db.host is required")
	}
	if c.Database.Port < 1 || c.Database.Port > 65535 {
		return fmt.Errorf("db.port out of range: %d", c.Database.Port)
	}
	if c.Database.Username == "" {
		return fmt.Errorf("db.username is required")
	}
	if c.Database.Database == "" {
		return fmt.Errorf("db.database is required")
	}
	if len(c.Clickhouse.Addr) == 0 {
		return fmt.Errorf("clickhouse.addr is required")
	}
	return nil
}

// SetConfigValue dynamically sets a string config value by key path (thread-safe).
// For numeric fields like smtp.port, use SetConfigInt instead.
func SetConfigValue(key string, value string) {
	mu.Lock()
	defer mu.Unlock()
	applyString(Conf, key, value)
}

// SetConfigInt sets an integer config value by key path (thread-safe).
func SetConfigInt(key string, value int) {
	mu.Lock()
	defer mu.Unlock()
	applyInt(Conf, key, value)
}

// applyString sets a dot-separated key path to a string value on the config struct.
func applyString(cfg *Config, key string, value string) {
	switch key {
	case "base_url":
		cfg.BaseURL = value
	case "smtp.host":
		cfg.SMTP.Host = value
	case "smtp.username":
		cfg.SMTP.Username = value
	case "smtp.password":
		cfg.SMTP.Password = value
	case "smtp.from":
		cfg.SMTP.From = value
	}
}

// applyInt sets a dot-separated key path to an int value on the config struct.
func applyInt(cfg *Config, key string, value int) {
	switch key {
	case "smtp.port":
		cfg.SMTP.Port = value
	}
}
