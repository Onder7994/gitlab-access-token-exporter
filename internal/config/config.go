package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	GitLab  GitLabConfig  `yaml:"gitlab"`
	HTTP    HTTPConfig    `yaml:"http"`
	Scan    ScanConfig    `yaml:"scan"`
	Metrics MetricsConfig `yaml:"metrics"`
}

type GitLabConfig struct {
	BaseURL    string `yaml:"base_url"`
	Token      string `yaml:"token"`
	Group      string `yaml:"group"`
	WithShared bool   `yaml:"with_shared"`
}

type HTTPConfig struct {
	TimeoutSeconds int `yaml:"timeout_seconds"`
}

type ScanConfig struct {
	IntervalSeconds int `yaml:"interval_seconds"`
	Concurrency     int `yaml:"concurrency"`
}

type MetricsConfig struct {
	Listen string `yaml:"listen"`
	Path   string `yaml:"path"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		HTTP: HTTPConfig{
			TimeoutSeconds: 30,
		},
		Scan: ScanConfig{
			IntervalSeconds: 300,
			Concurrency:     5,
		},
		Metrics: MetricsConfig{
			Listen: ":9108",
			Path:   "/metrics",
		},
		GitLab: GitLabConfig{
			WithShared: false,
		},
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}

	applyEnvOverrides(cfg)

	if cfg.GitLab.BaseURL == "" {
		return nil, fmt.Errorf("gitlab.base_url is required")
	}
	if cfg.GitLab.Group == "" {
		return nil, fmt.Errorf("gitlab.group is required")
	}
	if cfg.GitLab.Token == "" {
		return nil, fmt.Errorf("gitlab.token or GITLAB_TOKEN is required")
	}
	if cfg.Scan.Concurrency <= 0 {
		cfg.Scan.Concurrency = 5
	}
	if cfg.HTTP.TimeoutSeconds <= 0 {
		cfg.HTTP.TimeoutSeconds = 30
	}
	if cfg.Scan.IntervalSeconds <= 0 {
		cfg.Scan.IntervalSeconds = 300
	}
	if cfg.Metrics.Listen == "" {
		cfg.Metrics.Listen = ":9108"
	}
	if cfg.Metrics.Path == "" {
		cfg.Metrics.Path = "/metrics"
	}

	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	cfg.GitLab.BaseURL = getEnvString("GITLAB_BASE_URL", cfg.GitLab.BaseURL)
	cfg.GitLab.Token = getEnvString("GITLAB_TOKEN", cfg.GitLab.Token)
	cfg.GitLab.Group = getEnvString("GITLAB_GROUP", cfg.GitLab.Group)
	cfg.GitLab.WithShared = getEnvBool("GITLAB_WITH_SHARED", cfg.GitLab.WithShared)

	cfg.HTTP.TimeoutSeconds = getEnvInt("HTTP_TIMEOUT_SECONDS", cfg.HTTP.TimeoutSeconds)

	cfg.Scan.IntervalSeconds = getEnvInt("SCAN_INTERVAL_SECONDS", cfg.Scan.IntervalSeconds)
	cfg.Scan.Concurrency = getEnvInt("SCAN_CONCURRENCY", cfg.Scan.Concurrency)

	cfg.Metrics.Listen = getEnvString("METRICS_LISTEN", cfg.Metrics.Listen)
	cfg.Metrics.Path = getEnvString("METRICS_PATH", cfg.Metrics.Path)
}

func getEnvString(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}

	return n
}

func getEnvBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}

	return b
}
