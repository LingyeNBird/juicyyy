package main

type requestMode string

const (
	requestModeCompatible requestMode = "chatgpt_compatible"
	requestModeResponses  requestMode = "chatgpt_response"
)

type provider struct {
	BaseURL string   `json:"base_url"`
	APIKey  string   `json:"api_key"`
	Models  []string `json:"models"`
}

type requestSettings struct {
	Prompt         string      `json:"prompt"`
	TimeoutSeconds int         `json:"timeout_seconds"`
	Mode           requestMode `json:"mode"`
	RetryCount     int         `json:"retry_count"`
}

type appConfig struct {
	Providers       []provider      `json:"providers"`
	RequestSettings requestSettings `json:"request_settings"`
}

type modelResult struct {
	Model string
	Value string
	Error string
}
