package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigMissingFileReturnsNotExist(t *testing.T) {
	_, err := loadConfig(filepath.Join(t.TempDir(), "missing.json"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestLoadConfigEmptyFileReturnsEmptyConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("write empty file: %v", err)
	}

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Providers) != 0 {
		t.Fatalf("expected no providers, got %d", len(cfg.Providers))
	}
	if cfg.RequestSettings != defaultRequestSettings() {
		t.Fatalf("expected default request settings, got %+v", cfg.RequestSettings)
	}
}

func TestLoadConfigNormalizesBaseURLAndModels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.json")
	content := `{
	  "providers": [
	    {
	      "base_url": " https://example.com/v1/ ",
	      "api_key": "secret",
	      "models": [" gpt-4o-mini ", "gpt-4o-mini", "", "qwen-max "]
	    }
	  ]
	}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(cfg.Providers))
	}
	provider := cfg.Providers[0]
	if provider.BaseURL != "https://example.com/v1" {
		t.Fatalf("unexpected base URL: %q", provider.BaseURL)
	}
	wantModels := []string{"gpt-4o-mini", "qwen-max"}
	if len(provider.Models) != len(wantModels) {
		t.Fatalf("unexpected models length: got %d want %d", len(provider.Models), len(wantModels))
	}
	for i := range wantModels {
		if provider.Models[i] != wantModels[i] {
			t.Fatalf("unexpected model at %d: got %q want %q", i, provider.Models[i], wantModels[i])
		}
	}
}

func TestLoadConfigInvalidJSONReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("write broken config: %v", err)
	}

	if _, err := loadConfig(path); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestSaveConfigWritesIndentedJSONWithTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "providers.json")
	cfg := appConfig{
		Providers: []provider{{
			BaseURL: "https://example.com/v1",
			APIKey:  "secret",
			Models:  []string{"gpt-4o-mini", "qwen-max"},
		}},
	}

	if err := saveConfig(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	got := string(data)
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected trailing newline, got %q", got)
	}
	if !strings.Contains(got, "\n  \"providers\": [\n") {
		t.Fatalf("expected indented JSON, got %q", got)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("expected parent directory to exist: %v", err)
	}
}

func TestSaveConfigPersistsRequestSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "request-settings.json")
	cfg := appConfig{
		RequestSettings: requestSettings{
			Prompt:         "edited juicy prompt",
			TimeoutSeconds: 240,
			Mode:           requestModeResponses,
			RetryCount:     2,
		},
	}

	if err := saveConfig(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.RequestSettings != cfg.RequestSettings {
		t.Fatalf("unexpected request settings: got %+v want %+v", loaded.RequestSettings, cfg.RequestSettings)
	}
}

func TestSplitModelsSupportsCommasAndNewLines(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "new lines",
			raw:  "gpt-4o-mini\nqwen-max\nclaude-3.5-sonnet",
			want: []string{"gpt-4o-mini", "qwen-max", "claude-3.5-sonnet"},
		},
		{
			name: "mixed separators with blanks and duplicates",
			raw:  " gpt-4o-mini,\nqwen-max\r\n\nclaude-3.5-sonnet, qwen-max , , gpt-4o-mini ",
			want: []string{"gpt-4o-mini", "qwen-max", "claude-3.5-sonnet"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitModels(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("unexpected models length: got %d want %d (%q)", len(got), len(tt.want), tt.raw)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("unexpected model at %d: got %q want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{name: "empty", in: "   ", wantErr: "base URL is required"},
		{name: "missing scheme", in: "example.com/v1", wantErr: "base URL must include scheme and host"},
		{name: "missing host", in: "https:///v1", wantErr: "base URL must include scheme and host"},
		{name: "normal", in: "https://example.com/v1", want: "https://example.com/v1"},
		{name: "trims trailing slash", in: "https://example.com/v1/", want: "https://example.com/v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeBaseURL(tt.in)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected URL: got %q want %q", got, tt.want)
			}
		})
	}
}
