package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

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
	Titles        TitleConfig
}

// TitleConfig points the title worker at an OpenAI-compatible chat-completions
// API. The worker is off when APIURL is empty.
type TitleConfig struct {
	APIURL          string
	APIKey          string
	Model           string
	ReasoningEffort string
}

func (c TitleConfig) Enabled() bool { return c.APIURL != "" }

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
	config.Titles, err = loadTitleConfig()
	if err != nil {
		return ServerConfig{}, err
	}
	return config, nil
}

func loadTitleConfig() (TitleConfig, error) {
	titles := TitleConfig{
		APIURL:          strings.TrimRight(os.Getenv("TRAICR_TITLE_API_URL"), "/"),
		APIKey:          os.Getenv("TRAICR_TITLE_API_KEY"),
		Model:           os.Getenv("TRAICR_TITLE_MODEL"),
		ReasoningEffort: os.Getenv("TRAICR_TITLE_REASONING_EFFORT"),
	}
	if !titles.Enabled() {
		if titles.APIKey != "" || titles.Model != "" || titles.ReasoningEffort != "" {
			return TitleConfig{}, errors.New("TRAICR_TITLE_API_URL is required when other TRAICR_TITLE_* variables are set")
		}
		return titles, nil
	}
	parsed, err := url.Parse(titles.APIURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return TitleConfig{}, errors.New("TRAICR_TITLE_API_URL must be an absolute http or https URL without credentials, query, or fragment")
	}
	if titles.Model == "" {
		return TitleConfig{}, errors.New("TRAICR_TITLE_MODEL is required when TRAICR_TITLE_API_URL is set")
	}
	return titles, nil
}

func environmentOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
