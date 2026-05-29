package config

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/joho/godotenv/autoload"
	"github.com/spf13/viper"
)

var (
	//go:embed config_dev.yaml
	DevConfig []byte

	//go:embed config_prod.yaml
	ProdConfig []byte
)

type Config struct {
	Scheme struct {
		Address  string `mapstructure:"address"`
		HttpPort int    `mapstructure:"http_port"`
	} `mapstructure:"scheme"`

	Port              int    `mapstructure:"port"`
	PoolSize          int    `mapstructure:"pool_size"`
	LogLevel          string `mapstructure:"log_level"`
	SecretKey         string `mapstructure:"secret_key"`
	AppDebug          bool   `mapstructure:"app_debug"`
	DataPath          string `mapstructure:"data_path"`
	MaxmindLicenseKey string `mapstructure:"maxmind_license_key"`

	DefaultUser struct {
		Username string `mapstructure:"username"`
		Email    string `mapstructure:"email"`
		Password string `mapstructure:"password"`
	} `mapstructure:"default_user"`

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
	Conf *Config
)

func init() {
	Conf = new(Config)

	currentEnv := os.Getenv("APP_ENV")
	if currentEnv == "" {
		currentEnv = "dev"
	}
	slog.Info("current env", "env", currentEnv)
	var yamlContent []byte
	var configFileName string
	switch currentEnv {
	case "dev":
		yamlContent = DevConfig
		configFileName = "config_dev"
	case "prod":
		yamlContent = ProdConfig
		configFileName = "config_prod"
	default:
		slog.Error("Unsupported environment", "env", currentEnv)
		os.Exit(1)
	}

	configFile := fmt.Sprintf("./config/%s.yaml", configFileName)

	viper.SetConfigName(configFileName)
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	// bind env to config
	viper.AutomaticEnv()
	viper.SetEnvPrefix("ZENSTATS")

	bindEnvVars()

	err := viper.ReadInConfig()

	if err != nil {
		r := bytes.NewReader(yamlContent)
		if err := viper.ReadConfig(r); err != nil {
			panic(err) // 如果内嵌配置也加载失败，程序应该panic
		}
	}

	err = viper.Unmarshal(Conf)
	if err != nil {
		slog.Info("解析配置文件失败")
		panic(err)
	}

	_, err = os.Stat(configFile)
	fileExist := err == nil || os.IsExist(err)
	if !fileExist {
		if err := os.MkdirAll(filepath.Dir(configFile), 0766); err != nil {
			panic(err)
		}

		f, err := os.Create(configFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		if err := viper.WriteConfig(); err != nil {
			panic(err)
		}
	}

	viper.WatchConfig()
}

func bindEnvVars() {
	viper.BindEnv("db.host", "ZENSTATS_DB_HOST")
	viper.BindEnv("db.password", "ZENSTATS_DB_PASSWORD")
	viper.BindEnv("db.user", "ZENSTATS_DB_USERNAME")

	viper.BindEnv("clickhouse.addr", "ZENSTATS_CLICKHOUSE_ADDR")
	viper.BindEnv("clickhouse.password", "ZENSTATS_CLICKHOUSE_PASSWORD")
	viper.BindEnv("clickhouse.username", "ZENSTATS_CLICKHOUSE_USERNAME")

	viper.BindEnv("maxmind_license_key", "ZENSTATS_MAXMIND_LICENSE_KEY")

	if addrStr := os.Getenv("ZENSTATS_CLICKHOUSE_ADDR"); addrStr != "" {
		// try json
		var addr []string
		if err := json.Unmarshal([]byte(addrStr), &addr); err == nil {
			viper.Set("clickhouse.addr", addr)
		} else {
			viper.Set("clickhouse.addr", strings.Split(addrStr, ","))
		}
	}
}
