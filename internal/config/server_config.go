// Package config loads and validates Traicr process configuration.
package config

import (
	"errors"
	"os"
)

const (
	defaultListenAddress = ":8080"
	defaultDataDir       = "/data"
)

// ServerConfig contains the server settings available in the foundation release.
type ServerConfig struct {
	ListenAddress string
	DataDir       string
	AdminToken    string
}

// LoadServerConfig reads server settings from the environment and requires an admin token.
func LoadServerConfig() (ServerConfig, error) {
	config := ServerConfig{
		ListenAddress: environmentOrDefault("TRAICR_LISTEN_ADDRESS", defaultListenAddress),
		DataDir:       environmentOrDefault("TRAICR_DATA_DIR", defaultDataDir),
		AdminToken:    os.Getenv("TRAICR_ADMIN_TOKEN"),
	}

	if config.AdminToken == "" {
		return ServerConfig{}, errors.New("server configuration: TRAICR_ADMIN_TOKEN is required")
	}

	return config, nil
}

func environmentOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
