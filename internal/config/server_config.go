package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/regutierrez/traicr/internal/archive"
)

const (
	defaultListenAddress = ":8080"
	defaultDataDir       = "/data"
)

type ServerConfig struct {
	ListenAddress string
	DataDir       string
	AdminToken    string
	SecureCookies bool
	ArchiveLimits archive.Limits
}

func LoadServerConfig() (ServerConfig, error) {
	config := ServerConfig{
		ListenAddress: environmentOrDefault("TRAICR_LISTEN_ADDRESS", defaultListenAddress),
		DataDir:       environmentOrDefault("TRAICR_DATA_DIR", defaultDataDir),
		AdminToken:    os.Getenv("TRAICR_ADMIN_TOKEN"),
		ArchiveLimits: archive.DefaultLimits(),
	}

	if config.AdminToken == "" {
		return ServerConfig{}, errors.New("server configuration: TRAICR_ADMIN_TOKEN is required")
	}

	var err error
	config.SecureCookies, err = strconv.ParseBool(environmentOrDefault("TRAICR_SECURE_COOKIES", "false"))
	if err != nil {
		return ServerConfig{}, errors.New("TRAICR_SECURE_COOKIES must be true or false")
	}
	for name, target := range map[string]*int64{
		"TRAICR_MAX_UPLOAD_BYTES":   &config.ArchiveLimits.ArchiveBytes,
		"TRAICR_MAX_EXPANDED_BYTES": &config.ArchiveLimits.ExpandedBytes,
		"TRAICR_MAX_FILE_BYTES":     &config.ArchiveLimits.FileBytes,
	} {
		if value := os.Getenv(name); value != "" {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil || parsed <= 0 || parsed > 1<<50 {
				return ServerConfig{}, fmt.Errorf("%s must be a positive byte count no larger than 1 PiB", name)
			}
			*target = parsed
		}
	}
	return config, nil
}

func environmentOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
