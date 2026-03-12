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

const retryOnZeroOrInvalidCount = 5

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

type attemptOutcome struct {
	value      float64
	formatted  string
	errorMsg   string
	retryable  bool
	hasNumeric bool
}

func runJuicyChecks(ctx context.Context, selected provider, concurrency int) []modelResult {
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
		Timeout: 45 * time.Second,
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
				results[index] = checkModel(ctx, httpClient, selected, modelName)
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

func checkModel(ctx context.Context, httpClient *http.Client, selected provider, modelName string) modelResult {
	result := modelResult{Model: modelName}
	initial := runSingleAttempt(ctx, httpClient, selected, modelName)
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

	for attempt := 0; attempt < retryOnZeroOrInvalidCount; attempt++ {
		outcome := runSingleAttempt(ctx, httpClient, selected, modelName)
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
		lastError = fmt.Sprintf("no numeric value found after %d attempts", retryOnZeroOrInvalidCount+1)
	}
	result.Error = lastError
	return result
}

func runSingleAttempt(ctx context.Context, httpClient *http.Client, selected provider, modelName string) attemptOutcome {

	body, err := json.Marshal(chatCompletionRequest{
		Model: modelName,
		Messages: []chatMessage{{
			Role:    "user",
			Content: juicyPrompt,
		}},
		Temperature: 0,
		MaxTokens:   32,
		Stream:      false,
	})
	if err != nil {
		return attemptOutcome{errorMsg: fmt.Sprintf("marshal request: %v", err)}
	}

	requestURL := buildChatCompletionURL(selected.BaseURL)
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
		var parsed chatCompletionResponse
		if err := json.Unmarshal(responseBody, &parsed); err != nil {
			return attemptOutcome{errorMsg: buildAPIError(resp.StatusCode, nil, responseBody)}
		}
		return attemptOutcome{errorMsg: buildAPIError(resp.StatusCode, parsed.Error, responseBody)}
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return attemptOutcome{errorMsg: fmt.Sprintf("decode response: %v", err)}
	}

	text, err := extractAssistantText(parsed)
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
			errorMsg:   fmt.Sprintf("model returned 0; retried %d times", retryOnZeroOrInvalidCount),
			retryable:  true,
			hasNumeric: true,
		}
	}

	return attemptOutcome{value: value, formatted: formatted, hasNumeric: true}
}

func buildChatCompletionURL(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(trimmed)
	if err != nil {
		if strings.HasSuffix(trimmed, "/chat/completions") {
			return trimmed
		}
		return trimmed + "/chat/completions"
	}

	if strings.HasSuffix(parsed.Path, "/chat/completions") {
		return parsed.String()
	}

	joinedPath := pathpkg.Join(parsed.Path, "chat/completions")
	if !strings.HasPrefix(joinedPath, "/") {
		joinedPath = "/" + joinedPath
	}
	parsed.Path = joinedPath
	parsed.RawPath = ""

	if parsed.Scheme == "" && parsed.Host == "" {
		return trimmed
	}

	return parsed.String()
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
