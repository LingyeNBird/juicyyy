package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	pathpkg "path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const juicyPrompt = "你的juicy number是多少？直接回答数字"

var numberPattern = regexp.MustCompile(`[-+]?\d+(?:\.\d+)?`)

type chatCompletionRequest struct {
	Model       string          `json:"model"`
	Messages    []chatMessage   `json:"messages"`
	Temperature float64         `json:"temperature"`
	MaxTokens   int             `json:"max_tokens"`
	Stream      bool            `json:"stream"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responsesRequest struct {
	Model string        `json:"model"`
	Input []chatMessage `json:"input"`
}

type chatCompletionResponse struct {
	Choices []chatCompletionChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type chatCompletionChoice struct {
	Message chatCompletionChoiceMessage `json:"message"`
	Text    string                      `json:"text"`
}

type chatCompletionChoiceMessage struct {
	Content json.RawMessage `json:"content"`
}

type contentPart struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	Content string `json:"content"`
}

type responsesResponse struct {
	Output []responsesOutputItem `json:"output"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type responsesOutputItem struct {
	Type    string                 `json:"type"`
	Role    string                 `json:"role"`
	Content []responsesContentPart `json:"content"`
}

type responsesContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type attemptOutcome struct {
	value      float64
	formatted  string
	errorMsg   string
	retryable  bool
	hasNumeric bool
}

func runJuicyChecks(ctx context.Context, selected provider, settings requestSettings, concurrency int) []modelResult {
	settings = normalizeRequestSettings(settings)
	if concurrency < 1 {
		concurrency = 1
	}

	results := make([]modelResult, len(selected.Models))
	if len(selected.Models) == 0 {
		return results
	}

	workerCount := concurrency
	if workerCount > len(selected.Models) {
		workerCount = len(selected.Models)
	}

	jobs := make(chan int)
	httpClient := &http.Client{
		Timeout: time.Duration(settings.TimeoutSeconds) * time.Second,
		Transport: &http.Transport{
			MaxConnsPerHost:     workerCount,
			MaxIdleConns:        workerCount * 2,
			MaxIdleConnsPerHost: workerCount,
		},
	}
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				modelName := selected.Models[index]
				results[index] = checkModel(ctx, httpClient, selected, modelName, settings)
			}
		}()
	}

	for i := range selected.Models {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return results
}

func checkModel(ctx context.Context, httpClient *http.Client, selected provider, modelName string, settings requestSettings) modelResult {
	settings = normalizeRequestSettings(settings)
	result := modelResult{Model: modelName}
	initial := runSingleAttempt(ctx, httpClient, selected, modelName, settings)
	if !initial.retryable {
		if initial.hasNumeric {
			result.Value = initial.formatted
		} else {
			result.Error = initial.errorMsg
		}
		return result
	}

	bestValue := math.Inf(-1)
	bestFormatted := ""
	lastError := initial.errorMsg

	if initial.hasNumeric {
		bestValue = initial.value
		bestFormatted = initial.formatted
	}

	for attempt := 0; attempt < settings.RetryCount; attempt++ {
		outcome := runSingleAttempt(ctx, httpClient, selected, modelName, settings)
		if outcome.hasNumeric && outcome.value > bestValue {
			bestValue = outcome.value
			bestFormatted = outcome.formatted
		}
		if strings.TrimSpace(outcome.errorMsg) != "" {
			lastError = outcome.errorMsg
		}
	}

	if bestFormatted != "" {
		result.Value = bestFormatted
		return result
	}

	if strings.TrimSpace(lastError) == "" {
		lastError = fmt.Sprintf("no numeric value found after %d attempts", settings.RetryCount+1)
	}
	result.Error = lastError
	return result
}

func runSingleAttempt(ctx context.Context, httpClient *http.Client, selected provider, modelName string, settings requestSettings) attemptOutcome {
	settings = normalizeRequestSettings(settings)
	body, err := buildRequestBody(modelName, settings)
	if err != nil {
		return attemptOutcome{errorMsg: fmt.Sprintf("marshal request: %v", err)}
	}

	requestURL := buildRequestURL(selected.BaseURL, settings.Mode)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return attemptOutcome{errorMsg: fmt.Sprintf("create request: %v", err)}
	}

	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(selected.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(selected.APIKey))
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return attemptOutcome{errorMsg: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return attemptOutcome{errorMsg: fmt.Sprintf("read response: %v", err)}
	}

	if resp.StatusCode >= http.StatusBadRequest {
		var parsed struct {
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(responseBody, &parsed); err != nil {
			return attemptOutcome{errorMsg: buildAPIError(resp.StatusCode, nil, responseBody)}
		}
		return attemptOutcome{errorMsg: buildAPIError(resp.StatusCode, parsed.Error, responseBody)}
	}

	text, err := extractResponseText(responseBody, settings.Mode)
	if err != nil {
		return attemptOutcome{errorMsg: err.Error(), retryable: true}
	}

	value, formatted, err := parseNumericValue(text)
	if err != nil {
		return attemptOutcome{errorMsg: err.Error(), retryable: true}
	}

	if value == 0 {
		return attemptOutcome{
			value:      value,
			formatted:  formatted,
			errorMsg:   fmt.Sprintf("model returned 0; retried %d times", settings.RetryCount),
			retryable:  true,
			hasNumeric: true,
		}
	}

	return attemptOutcome{value: value, formatted: formatted, hasNumeric: true}
}

func buildRequestBody(modelName string, settings requestSettings) ([]byte, error) {
	if settings.Mode == requestModeResponses {
		return json.Marshal(responsesRequest{
			Model: modelName,
			Input: []chatMessage{{
				Role:    "user",
				Content: settings.Prompt,
			}},
		})
	}

	return json.Marshal(chatCompletionRequest{
		Model: modelName,
		Messages: []chatMessage{{
			Role:    "user",
			Content: settings.Prompt,
		}},
		Temperature: 0,
		MaxTokens:   32,
		Stream:      false,
	})
}

func buildRequestURL(baseURL string, mode requestMode) string {
	if mode == requestModeResponses {
		return buildResponsesURL(baseURL)
	}
	return buildChatCompletionURL(baseURL)
}

func buildChatCompletionURL(baseURL string) string {
	return buildAPIURL(baseURL, "chat/completions")
}

func buildResponsesURL(baseURL string) string {
	return buildAPIURL(baseURL, "responses")
}

func buildAPIURL(baseURL, endpoint string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fallbackAPIURL(trimmed, endpoint)
	}

	if strings.HasSuffix(parsed.Path, "/"+endpoint) {
		return parsed.String()
	}
	parsed.Path = swapEndpointPath(parsed.Path, endpoint)
	parsed.RawPath = ""

	if parsed.Scheme == "" && parsed.Host == "" {
		return fallbackAPIURL(trimmed, endpoint)
	}

	return parsed.String()
}

func swapEndpointPath(path, endpoint string) string {
	for _, candidate := range []string{"/chat/completions", "/responses"} {
		if strings.HasSuffix(path, candidate) {
			path = strings.TrimSuffix(path, candidate)
			break
		}
	}

	joinedPath := pathpkg.Join(path, endpoint)
	if !strings.HasPrefix(joinedPath, "/") {
		joinedPath = "/" + joinedPath
	}
	return joinedPath
}

func fallbackAPIURL(trimmed, endpoint string) string {
	for _, candidate := range []string{"/chat/completions", "/responses"} {
		if strings.HasSuffix(trimmed, candidate) {
			return strings.TrimSuffix(trimmed, candidate) + "/" + endpoint
		}
	}
	return trimmed + "/" + endpoint
}

func buildAPIError(statusCode int, apiErr *struct {
	Message string `json:"message"`
}, body []byte) string {
	if apiErr != nil && strings.TrimSpace(apiErr.Message) != "" {
		return fmt.Sprintf("API %d: %s", statusCode, strings.TrimSpace(apiErr.Message))
	}

	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) > 240 {
		trimmed = trimmed[:240] + "..."
	}
	if trimmed == "" {
		return fmt.Sprintf("API %d", statusCode)
	}

	return fmt.Sprintf("API %d: %s", statusCode, trimmed)
}

func extractAssistantText(response chatCompletionResponse) (string, error) {
	if len(response.Choices) == 0 {
		return "", errors.New("response has no choices")
	}

	choice := response.Choices[0]
	if strings.TrimSpace(choice.Text) != "" {
		return strings.TrimSpace(choice.Text), nil
	}

	raw := bytes.TrimSpace(choice.Message.Content)
	if len(raw) == 0 {
		return "", errors.New("response has empty message content")
	}

	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return "", fmt.Errorf("decode message content: %w", err)
		}
		return strings.TrimSpace(text), nil
	}

	var parts []contentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", fmt.Errorf("decode message content: %w", err)
	}

	chunks := make([]string, 0, len(parts))
	for _, part := range parts {
		text := strings.TrimSpace(part.Text)
		if text == "" {
			text = strings.TrimSpace(part.Content)
		}
		if text != "" {
			chunks = append(chunks, text)
		}
	}
	if len(chunks) == 0 {
		return "", errors.New("response content did not contain text")
	}

	return strings.Join(chunks, "\n"), nil
}

func extractResponsesText(response responsesResponse) (string, error) {
	chunks := make([]string, 0)
	for _, output := range response.Output {
		if output.Type != "message" || output.Role != "assistant" {
			continue
		}
		for _, part := range output.Content {
			if part.Type != "output_text" {
				continue
			}
			text := strings.TrimSpace(part.Text)
			if text != "" {
				chunks = append(chunks, text)
			}
		}
	}
	if len(chunks) == 0 {
		return "", errors.New("response content did not contain text")
	}
	return strings.Join(chunks, "\n"), nil
}

func extractResponseText(body []byte, mode requestMode) (string, error) {
	if mode == requestModeResponses {
		var parsed responsesResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return "", fmt.Errorf("decode response: %w", err)
		}
		return extractResponsesText(parsed)
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return extractAssistantText(parsed)
}

func parseNumericValue(text string) (float64, string, error) {
	match := numberPattern.FindString(strings.TrimSpace(text))
	if match == "" {
		return 0, "", fmt.Errorf("no numeric value found in %q", text)
	}

	value, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid numeric value %q", match)
	}

	formatted := strconv.FormatFloat(value, 'f', -1, 64)
	return value, formatted, nil
}
