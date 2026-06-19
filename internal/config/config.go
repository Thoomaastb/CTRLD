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
	Level  string `mapstructure:"level"`  // trace, debug, info, warn, error
	Format string `mapstructure:"format"` // json, pretty
}

// SecurityConfig enthält sicherheitsrelevante Einstellungen.
type SecurityConfig struct {
	ArgonMemory      uint32 `mapstructure:"argon_memory"`       // in KiB, default 64MB
	ArgonIterations  uint32 `mapstructure:"argon_iterations"`   // default 3
	ArgonParallelism uint8  `mapstructure:"argon_parallelism"`  // default 2
	JWTAccessTTLMin  int    `mapstructure:"jwt_access_ttl_min"` // default 15
	JWTRefreshTTLDay int    `mapstructure:"jwt_refresh_ttl_day"`// default 7
}

// DatabaseConfig konfiguriert SQLite.
type DatabaseConfig struct {
	Path string `mapstructure:"path"` // default /var/lib/ctrld/ctrld.db
}

// Load lädt die Konfiguration aus Datei und Umgebungsvariablen.
// Reihenfolge: Defaults → Datei → ENV (CTRLD_* Präfix)
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// Defaults setzen
	setDefaults(v)

	// Konfigurationsdatei
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("/etc/ctrld/")
		v.AddConfigPath("$HOME/.ctrld/")
		v.AddConfigPath(".")
	}

	// Umgebungsvariablen: CTRLD_SERVER_PORT → server.port
	v.SetEnvPrefix("CTRLD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		// Konfigurationsdatei ist optional — nur echte Fehler melden
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("config: fehler beim lesen der konfigurationsdatei: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: fehler beim parsen der konfiguration: %w", err)
	}

	return &cfg, nil
}

// setDefaults setzt sichere Standardwerte.
func setDefaults(v *viper.Viper) {
	// Server
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8443)
	v.SetDefault("server.read_timeout_sec", 10)
	v.SetDefault("server.write_timeout_sec", 30)
	v.SetDefault("server.idle_timeout_sec", 120)

	// Logging
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	// Security — Argon2id Empfehlungen (OWASP)
	v.SetDefault("security.argon_memory", 65536)  // 64 MB
	v.SetDefault("security.argon_iterations", 3)
	v.SetDefault("security.argon_parallelism", 2)
	v.SetDefault("security.jwt_access_ttl_min", 15)
	v.SetDefault("security.jwt_refresh_ttl_day", 7)

	// Database
	v.SetDefault("database.path", "/var/lib/ctrld/ctrld.db")
}
