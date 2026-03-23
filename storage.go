package main

import (
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func defaultRequestSettings() requestSettings {
	return requestSettings{
		Prompt:          juicyPrompt,
		IntervalSeconds: 0,
		TimeoutSeconds:  180,
		Mode:            requestModeCompatible,
		RetryCount:      5,
	}
}

func normalizeRequestSettings(settings requestSettings) requestSettings {
	defaults := defaultRequestSettings()
	settings.Prompt = strings.TrimSpace(settings.Prompt)
	if settings.Prompt == "" {
		settings.Prompt = defaults.Prompt
	}
	if math.IsNaN(settings.IntervalSeconds) || math.IsInf(settings.IntervalSeconds, 0) || settings.IntervalSeconds < 0 {
		settings.IntervalSeconds = defaults.IntervalSeconds
	}
	if settings.TimeoutSeconds <= 0 {
		settings.TimeoutSeconds = defaults.TimeoutSeconds
	}
	if settings.Mode != requestModeCompatible && settings.Mode != requestModeResponses {
		settings.Mode = defaults.Mode
	}
	if settings.RetryCount < 0 {
		settings.RetryCount = defaults.RetryCount
	}
	return settings
}

func normalizeConfig(cfg appConfig) appConfig {
	for i := range cfg.Providers {
		cfg.Providers[i].BaseURL = strings.TrimRight(strings.TrimSpace(cfg.Providers[i].BaseURL), "/")
		cfg.Providers[i].Models = splitModels(strings.Join(cfg.Providers[i].Models, ","))
	}
	if cfg.RequestSettings == (requestSettings{}) {
		cfg.RequestSettings = defaultRequestSettings()
	} else {
		cfg.RequestSettings = normalizeRequestSettings(cfg.RequestSettings)
	}
	return cfg
}

func loadConfig(path string) (appConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return appConfig{}, os.ErrNotExist
		}
		return appConfig{}, err
	}

	if len(data) == 0 {
		return normalizeConfig(appConfig{}), nil
	}

	cfg := appConfig{RequestSettings: defaultRequestSettings()}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return appConfig{}, err
	}

	return normalizeConfig(cfg), nil
}

func saveConfig(path string, cfg appConfig) error {
	cfg = normalizeConfig(cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func splitModels(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	models := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		model := strings.TrimSpace(part)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		models = append(models, model)
	}

	return models
}

func normalizeBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("base URL is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("base URL must include scheme and host")
	}

	return strings.TrimRight(trimmed, "/"), nil
}
