package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config enthält die gesamte Anwendungskonfiguration.
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Log      LogConfig      `mapstructure:"log"`
	Security SecurityConfig `mapstructure:"security"`
	Database DatabaseConfig `mapstructure:"database"`
}

// ServerConfig konfiguriert den HTTP-Server.
type ServerConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	TLSCertFile     string `mapstructure:"tls_cert_file"`
	TLSKeyFile      string `mapstructure:"tls_key_file"`
	ReadTimeoutSec  int    `mapstructure:"read_timeout_sec"`
	WriteTimeoutSec int    `mapstructure:"write_timeout_sec"`
	IdleTimeoutSec  int    `mapstructure:"idle_timeout_sec"`
}

// LogConfig konfiguriert das Logging.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// SecurityConfig enthält sicherheitsrelevante Einstellungen.
type SecurityConfig struct {
	JWTSecret        string `mapstructure:"jwt_secret"`        // min. 32 Zeichen — MUSS gesetzt werden
	ArgonMemory      uint32 `mapstructure:"argon_memory"`
	ArgonIterations  uint32 `mapstructure:"argon_iterations"`
	ArgonParallelism uint8  `mapstructure:"argon_parallelism"`
	JWTAccessTTLMin  int    `mapstructure:"jwt_access_ttl_min"`
	JWTRefreshTTLDay int    `mapstructure:"jwt_refresh_ttl_day"`
}

// DatabaseConfig konfiguriert SQLite.
type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

// Load lädt die Konfiguration aus Datei und Umgebungsvariablen.
func Load(cfgFile string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("/etc/ctrld/")
		v.AddConfigPath("$HOME/.ctrld/")
		v.AddConfigPath(".")
	}

	v.SetEnvPrefix("CTRLD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("config: fehler beim lesen: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: fehler beim parsen: %w", err)
	}

	// JWT-Secret Pflichtfeld prüfen
	if len(cfg.Security.JWTSecret) < 32 {
		return nil, fmt.Errorf("config: CTRLD_SECURITY_JWT_SECRET muss mindestens 32 zeichen haben")
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8443)
	v.SetDefault("server.read_timeout_sec", 10)
	v.SetDefault("server.write_timeout_sec", 30)
	v.SetDefault("server.idle_timeout_sec", 120)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("security.argon_memory", 65536)
	v.SetDefault("security.argon_iterations", 3)
	v.SetDefault("security.argon_parallelism", 2)
	v.SetDefault("security.jwt_access_ttl_min", 15)
	v.SetDefault("security.jwt_refresh_ttl_day", 7)
	// JWT-Secret hat keinen Default — muss explizit gesetzt werden
	v.SetDefault("security.jwt_secret", "")
	v.SetDefault("database.path", "/var/lib/ctrld/ctrld.db")
}
