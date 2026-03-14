package main

type provider struct {
	BaseURL string   `json:"base_url"`
	APIKey  string   `json:"api_key"`
	Models  []string `json:"models"`
}

type appConfig struct {
	Providers []provider `json:"providers"`
}

type modelResult struct {
	Model string
	Value string
	Error string
}
