package server

import "github.com/Thoomaastb/CTRLD/internal/config"

// defaultTestConfig gibt eine minimale Test-Konfiguration zurück.
// Nur für Tests — nicht für Produktion.
func defaultTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host:            "127.0.0.1",
			Port:            8443,
			ReadTimeoutSec:  10,
			WriteTimeoutSec: 30,
			IdleTimeoutSec:  120,
		},
		Log: config.LogConfig{
			Level:  "error",
			Format: "json",
		},
		Security: config.SecurityConfig{
			JWTSecret:        "test-secret-min-32-bytes-long-xx",
			ArgonMemory:      65536,
			ArgonIterations:  3,
			ArgonParallelism: 2,
			JWTAccessTTLMin:  15,
			JWTRefreshTTLDay: 7,
		},
		Database: config.DatabaseConfig{
			Path: ":memory:",
		},
	}
}
