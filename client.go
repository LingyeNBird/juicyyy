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
	"sync/atomic"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	openairesponses "github.com/openai/openai-go/v3/responses"
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

func runJuicyChecks(ctx context.Context, selected provider, settings requestSettings, concurrency int, onProgress func(completed, total int)) []modelResult {
	settings = normalizeRequestSettings(settings)
	if concurrency < 1 {
		concurrency = 1
	}

	results := make([]modelResult, len(selected.Models))
	totalModels := len(selected.Models)
	if totalModels == 0 {
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
	var completedChecks atomic.Int32

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				modelName := selected.Models[index]
				results[index] = checkModel(ctx, httpClient, selected, modelName, settings)
				if onProgress != nil {
					onProgress(int(completedChecks.Add(1)), totalModels)
				}
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
	requestURL := buildRequestURL(selected.BaseURL, settings.Mode)
	capture := &responseCapture{}
	sdkClient := newOpenAIClient(buildSDKClientOptions(requestURL, selected.APIKey, newCapturingHTTPClient(httpClient, capture))...)

	text, err, retryable := requestModelText(ctx, sdkClient, capture, modelName, settings)
	if err != nil {
		return attemptOutcome{errorMsg: err.Error(), retryable: retryable}
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

type responseCapture struct {
	statusCode int
	body       bytes.Buffer
}

func (c *responseCapture) bytes() []byte {
	if c == nil {
		return nil
	}
	return c.body.Bytes()
}

type responseCaptureTransport struct {
	base    http.RoundTripper
	capture *responseCapture
}

func (t responseCaptureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if t.capture == nil {
		return resp, nil
	}

	t.capture.statusCode = resp.StatusCode
	if resp.Body != nil {
		resp.Body = &captureReadCloser{ReadCloser: resp.Body, buffer: &t.capture.body}
	}
	return resp, nil
}

type captureReadCloser struct {
	io.ReadCloser
	buffer *bytes.Buffer
}

func (r *captureReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		_, _ = r.buffer.Write(p[:n])
	}
	return n, err
}

func newCapturingHTTPClient(base *http.Client, capture *responseCapture) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}

	clone := *base
	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	clone.Transport = responseCaptureTransport{base: transport, capture: capture}
	return &clone
}

func requestModelText(ctx context.Context, client openai.Client, capture *responseCapture, modelName string, settings requestSettings) (string, error, bool) {
	if settings.Mode == requestModeResponses {
		params := openairesponses.ResponseNewParams{
			Model: openai.ResponsesModel(modelName),
			Input: openairesponses.ResponseNewParamsInputUnion{
				OfInputItemList: openairesponses.ResponseInputParam{
					openairesponses.ResponseInputItemParamOfMessage(settings.Prompt, openairesponses.EasyInputMessageRoleUser),
				},
			},
		}

		response, err := client.Responses.New(ctx, params)
		if err != nil {
			return handleSDKRequestError(err, capture, settings.Mode)
		}

		text, textErr := extractSDKResponsesText(*response)
		if textErr == nil {
			return text, nil, false
		}
		if fallbackText, fallbackErr := extractFallbackResponseText(capture.bytes(), settings.Mode); fallbackErr == nil {
			return fallbackText, nil, false
		}
		return "", textErr, true
	}

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(modelName),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(settings.Prompt),
		},
		Temperature: openai.Float(0),
		MaxTokens:   openai.Int(32),
	}

	completion, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return handleSDKRequestError(err, capture, settings.Mode)
	}

	text, textErr := extractChatCompletionText(*completion)
	if textErr == nil {
		return text, nil, false
	}
	if fallbackText, fallbackErr := extractFallbackResponseText(capture.bytes(), settings.Mode); fallbackErr == nil {
		return fallbackText, nil, false
	}
	return "", textErr, true
}

func handleSDKRequestError(err error, capture *responseCapture, mode requestMode) (string, error, bool) {
	if capture != nil && capture.statusCode >= http.StatusBadRequest {
		var apiErr *openai.Error
		if errors.As(err, &apiErr) {
			return "", errors.New(buildAPIError(apiErr.StatusCode, &struct {
				Message string `json:"message"`
			}{Message: apiErr.Message}, capture.bytes())), false
		}
		return "", errors.New(buildAPIError(capture.statusCode, nil, capture.bytes())), false
	}

	if capture != nil && len(capture.bytes()) > 0 {
		if text, fallbackErr := extractFallbackResponseText(capture.bytes(), mode); fallbackErr == nil {
			return text, nil, false
		}
		return "", fmt.Errorf("decode response: %v", err), true
	}

	return "", fmt.Errorf("request failed: %v", err), false
}

func newOpenAIClient(opts ...option.RequestOption) openai.Client {
	return openai.Client{
		Options:   opts,
		Chat:      openai.NewChatService(opts...),
		Responses: openairesponses.NewResponseService(opts...),
	}
}

func buildSDKClientOptions(requestURL, apiKey string, httpClient *http.Client) []option.RequestOption {
	baseURL, queryOptions := buildSDKBaseURL(requestURL)
	trimmedAPIKey := strings.TrimSpace(apiKey)

	opts := []option.RequestOption{
		option.WithBaseURL(baseURL),
		option.WithHTTPClient(httpClient),
		option.WithMaxRetries(0),
	}
	if trimmedAPIKey != "" {
		opts = append(opts, option.WithAPIKey(trimmedAPIKey))
	}
	return append(opts, queryOptions...)
}

func buildSDKBaseURL(requestURL string) (string, []option.RequestOption) {
	parsed, err := url.Parse(requestURL)
	if err != nil {
		return requestURL, nil
	}

	queryOptions := make([]option.RequestOption, 0)
	for key, values := range parsed.Query() {
		for _, value := range values {
			queryOptions = append(queryOptions, option.WithQueryAdd(key, value))
		}
	}

	parsed.Path = stripEndpointPath(parsed.Path)
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	if parsed.Path == "" {
		parsed.Path = "/"
	}

	return parsed.String(), queryOptions
}

func stripEndpointPath(path string) string {
	for _, candidate := range []string{"/chat/completions", "/responses"} {
		if strings.HasSuffix(path, candidate) {
			return strings.TrimSuffix(path, candidate)
		}
	}
	return path
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

func extractChatCompletionText(response openai.ChatCompletion) (string, error) {
	if len(response.Choices) == 0 {
		return "", errors.New("response has no choices")
	}

	text := strings.TrimSpace(response.Choices[0].Message.Content)
	if text == "" {
		return "", errors.New("response has empty message content")
	}
	return text, nil
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

func extractSDKResponsesText(response openairesponses.Response) (string, error) {
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

func extractFallbackResponseText(body []byte, mode requestMode) (string, error) {
	if len(body) == 0 {
		return "", errors.New("response body was empty")
	}
	return extractResponseText(body, mode)
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
