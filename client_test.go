package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testRequestSettings() requestSettings {
	return defaultRequestSettings()
}

func TestSplitModels(t *testing.T) {
	got := splitModels("gpt-4o-mini, gpt-4o-mini, llama-3.1-8b , , qwen-max")
	want := []string{"gpt-4o-mini", "llama-3.1-8b", "qwen-max"}

	if len(got) != len(want) {
		t.Fatalf("unexpected length: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected model at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestBuildChatCompletionURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "base v1", in: "https://example.com/v1", want: "https://example.com/v1/chat/completions"},
		{name: "full endpoint", in: "https://example.com/v1/chat/completions", want: "https://example.com/v1/chat/completions"},
		{name: "query preserved", in: "https://example.com/openai/v1?api-version=2024-10-21", want: "https://example.com/openai/v1/chat/completions?api-version=2024-10-21"},
		{name: "swap responses endpoint", in: "https://example.com/v1/responses", want: "https://example.com/v1/chat/completions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildChatCompletionURL(tt.in); got != tt.want {
				t.Fatalf("unexpected url: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestBuildResponsesURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "base v1", in: "https://example.com/v1", want: "https://example.com/v1/responses"},
		{name: "full endpoint", in: "https://example.com/v1/responses", want: "https://example.com/v1/responses"},
		{name: "swap chat endpoint", in: "https://example.com/v1/chat/completions", want: "https://example.com/v1/responses"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildResponsesURL(tt.in); got != tt.want {
				t.Fatalf("unexpected url: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestParseNumericValue(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "plain integer", in: "42", want: "42"},
		{name: "text wrapper", in: "juicy is 7.5 today", want: "7.5"},
		{name: "signed number", in: "-3", want: "-3"},
		{name: "missing", in: "not available", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got, err := parseNumericValue(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected value: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestExtractAssistantText(t *testing.T) {
	t.Run("string content", func(t *testing.T) {
		response := chatCompletionResponse{
			Choices: []chatCompletionChoice{{
				Message: chatCompletionChoiceMessage{Content: json.RawMessage(`"18"`)},
			}},
		}

		got, err := extractAssistantText(response)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "18" {
			t.Fatalf("unexpected text: got %q", got)
		}
	})

	t.Run("array content", func(t *testing.T) {
		response := chatCompletionResponse{
			Choices: []chatCompletionChoice{{
				Message: chatCompletionChoiceMessage{Content: json.RawMessage(`[{"type":"text","text":"22.5"}]`)},
			}},
		}

		got, err := extractAssistantText(response)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "22.5" {
			t.Fatalf("unexpected text: got %q", got)
		}
	})
}

func TestCheckModelRetriesZeroAndKeepsHighest(t *testing.T) {
	responses := []string{"0", "2", "7.5", "4", "6", "5"}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		index := int(calls.Add(1)) - 1
		fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, responses[index])
	}))
	defer server.Close()

	result := checkModel(context.Background(), server.Client(), provider{BaseURL: server.URL}, "demo-model", testRequestSettings())

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Value != "7.5" {
		t.Fatalf("unexpected value: got %q want %q", result.Value, "7.5")
	}
	if got := calls.Load(); got != 6 {
		t.Fatalf("unexpected call count: got %d want 6", got)
	}
}

func TestCheckModelRetriesNonnumericAndKeepsHighest(t *testing.T) {
	responses := []string{"no idea", "3", "2", "4", "bad", "1"}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index := int(calls.Add(1)) - 1
		fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, responses[index])
	}))
	defer server.Close()

	result := checkModel(context.Background(), server.Client(), provider{BaseURL: server.URL}, "demo-model", testRequestSettings())

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Value != "4" {
		t.Fatalf("unexpected value: got %q want %q", result.Value, "4")
	}
	if got := calls.Load(); got != 6 {
		t.Fatalf("unexpected call count: got %d want 6", got)
	}
}

func TestCheckModelReturnsErrorWhenAllRetryableAttemptsFail(t *testing.T) {
	responses := []string{"0", "zero", "still zero", "0.0", "nothing", "bad output"}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index := int(calls.Add(1)) - 1
		fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, responses[index])
	}))
	defer server.Close()

	result := checkModel(context.Background(), server.Client(), provider{BaseURL: server.URL}, "demo-model", testRequestSettings())

	if result.Value != "0" {
		t.Fatalf("unexpected value: got %q want %q", result.Value, "0")
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if got := calls.Load(); got != 6 {
		t.Fatalf("unexpected call count: got %d want 6", got)
	}
}

func TestCheckModelDoesNotRetryAPIErrors(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"rate limit"}}`)
	}))
	defer server.Close()

	result := checkModel(context.Background(), server.Client(), provider{BaseURL: server.URL}, "demo-model", testRequestSettings())

	if result.Value != "" {
		t.Fatalf("unexpected value: %q", result.Value)
	}
	if result.Error != "API 429: rate limit" {
		t.Fatalf("unexpected error: got %q", result.Error)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("unexpected call count: got %d want 1", got)
	}
}

func TestRunJuicyChecksEmptyModels(t *testing.T) {
	results := runJuicyChecks(context.Background(), provider{BaseURL: "https://example.com", Models: nil}, testRequestSettings(), 5, nil)
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %d", len(results))
	}
}

func TestRunJuicyChecksPreservesModelOrder(t *testing.T) {
	selected := provider{
		BaseURL: serverURL(t, func(w http.ResponseWriter, r *http.Request) {
			var req chatCompletionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			switch req.Model {
			case "slow":
				time.Sleep(30 * time.Millisecond)
				fmt.Fprint(w, `{"choices":[{"message":{"content":"1"}}]}`)
			case "fast":
				fmt.Fprint(w, `{"choices":[{"message":{"content":"2"}}]}`)
			case "medium":
				time.Sleep(10 * time.Millisecond)
				fmt.Fprint(w, `{"choices":[{"message":{"content":"3"}}]}`)
			default:
				t.Fatalf("unexpected model %q", req.Model)
			}
		}),
		Models: []string{"slow", "fast", "medium"},
	}

	results := runJuicyChecks(context.Background(), selected, testRequestSettings(), 3, nil)
	if len(results) != 3 {
		t.Fatalf("unexpected results length: got %d want 3", len(results))
	}
	want := []modelResult{{Model: "slow", Value: "1"}, {Model: "fast", Value: "2"}, {Model: "medium", Value: "3"}}
	for i := range want {
		if results[i] != want[i] {
			t.Fatalf("unexpected result at %d: got %+v want %+v", i, results[i], want[i])
		}
	}
}

func TestRunJuicyChecksReportsProgressForSuccessAndFailure(t *testing.T) {
	selected := provider{
		BaseURL: serverURL(t, func(w http.ResponseWriter, r *http.Request) {
			var req chatCompletionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			switch req.Model {
			case "ok-model":
				fmt.Fprint(w, `{"choices":[{"message":{"content":"7"}}]}`)
			case "bad-model":
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, `{"error":{"message":"rate limit"}}`)
			default:
				t.Fatalf("unexpected model %q", req.Model)
			}
		}),
		Models: []string{"ok-model", "bad-model"},
	}

	var progressCalls atomic.Int32
	var maxCompleted atomic.Int32
	var seenTotal atomic.Int32

	results := runJuicyChecks(context.Background(), selected, testRequestSettings(), 2, func(completed, total int) {
		progressCalls.Add(1)
		seenTotal.Store(int32(total))
		for {
			current := maxCompleted.Load()
			if int32(completed) <= current || maxCompleted.CompareAndSwap(current, int32(completed)) {
				break
			}
		}
	})

	if got := progressCalls.Load(); got != 2 {
		t.Fatalf("expected 2 progress callbacks, got %d", got)
	}
	if got := maxCompleted.Load(); got != 2 {
		t.Fatalf("expected progress to reach 2 completed models, got %d", got)
	}
	if got := seenTotal.Load(); got != 2 {
		t.Fatalf("expected progress total 2, got %d", got)
	}
	if len(results) != 2 {
		t.Fatalf("unexpected results length: got %d want 2", len(results))
	}
	if results[0] != (modelResult{Model: "ok-model", Value: "7"}) {
		t.Fatalf("unexpected first result: %+v", results[0])
	}
	if results[1] != (modelResult{Model: "bad-model", Error: "API 429: rate limit"}) {
		t.Fatalf("unexpected second result: %+v", results[1])
	}
}

func TestRunSingleAttemptSetsAuthorizationHeaderWhenAPIKeyPresent(t *testing.T) {
	selected := provider{BaseURL: serverURL(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		var req chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got := req.Messages[0].Content; got != "custom prompt" {
			t.Fatalf("unexpected prompt content: %q", got)
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"9"}}]}`)
	}), APIKey: "secret"}

	settings := testRequestSettings()
	settings.Prompt = "custom prompt"
	outcome := runSingleAttempt(context.Background(), http.DefaultClient, selected, "demo-model", settings)
	if !outcome.hasNumeric || outcome.formatted != "9" {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
}

func TestRunSingleAttemptOmitsAuthorizationHeaderWhenAPIKeyEmpty(t *testing.T) {
	selected := provider{BaseURL: serverURL(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("expected empty authorization header, got %q", got)
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"11"}}]}`)
	})}

	outcome := runSingleAttempt(context.Background(), http.DefaultClient, selected, "demo-model", testRequestSettings())
	if !outcome.hasNumeric || outcome.formatted != "11" {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
}

func TestRunSingleAttemptOmitsAuthorizationHeaderWhenEnvAPIKeyExists(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-secret")
	selected := provider{BaseURL: serverURL(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("expected empty authorization header, got %q", got)
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"14"}}]}`)
	})}

	outcome := runSingleAttempt(context.Background(), http.DefaultClient, selected, "demo-model", testRequestSettings())
	if !outcome.hasNumeric || outcome.formatted != "14" {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
}

func TestRunSingleAttemptPreservesQueryStringWithSDKBaseURL(t *testing.T) {
	selected := provider{BaseURL: serverURL(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("api-version"); got != "2024-10-21" {
			t.Fatalf("unexpected api-version query: %q", got)
		}
		if got := r.URL.Query().Get("tenant"); got != "demo" {
			t.Fatalf("unexpected tenant query: %q", got)
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"15"}}]}`)
	}) + "/openai/v1/responses?api-version=2024-10-21&tenant=demo"}

	outcome := runSingleAttempt(context.Background(), http.DefaultClient, selected, "demo-model", testRequestSettings())
	if !outcome.hasNumeric || outcome.formatted != "15" {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
}

func TestRunSingleAttemptSupportsLegacyChatCompletionTextField(t *testing.T) {
	selected := provider{BaseURL: serverURL(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"text":"16"}]}`)
	})}

	outcome := runSingleAttempt(context.Background(), http.DefaultClient, selected, "demo-model", testRequestSettings())
	if !outcome.hasNumeric || outcome.formatted != "16" {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
}

func TestRunSingleAttemptSupportsLegacyChatCompletionArrayContent(t *testing.T) {
	selected := provider{BaseURL: serverURL(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"content":[{"type":"text","text":"22.5"}]}}]}`)
	})}

	outcome := runSingleAttempt(context.Background(), http.DefaultClient, selected, "demo-model", testRequestSettings())
	if !outcome.hasNumeric || outcome.formatted != "22.5" {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
}

func TestRunSingleAttemptSupportsResponsesMode(t *testing.T) {
	selected := provider{BaseURL: serverURL(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req responsesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got := req.Input[0].Content; got != "custom prompt" {
			t.Fatalf("unexpected prompt content: %q", got)
		}
		fmt.Fprint(w, `{"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"12"}]}]}`)
	})}
	settings := testRequestSettings()
	settings.Mode = requestModeResponses
	settings.Prompt = "custom prompt"

	outcome := runSingleAttempt(context.Background(), http.DefaultClient, selected, "demo-model", settings)
	if !outcome.hasNumeric || outcome.formatted != "12" {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
}

func TestBuildAPIErrorFallsBackToTrimmedBody(t *testing.T) {
	body := []byte(strings.Repeat("x", 300))
	got := buildAPIError(http.StatusBadGateway, nil, body)
	if !strings.HasPrefix(got, "API 502: ") {
		t.Fatalf("unexpected prefix: %q", got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected trimmed suffix, got %q", got)
	}
}

func TestExtractAssistantTextErrorsOnEmptyContent(t *testing.T) {
	response := chatCompletionResponse{
		Choices: []chatCompletionChoice{{
			Message: chatCompletionChoiceMessage{Content: json.RawMessage(`[]`)},
		}},
	}

	_, err := extractAssistantText(response)
	if err == nil || err.Error() != "response content did not contain text" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func serverURL(t *testing.T, handler func(http.ResponseWriter, *http.Request)) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL
}
