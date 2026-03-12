package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildChatCompletionURL(tt.in); got != tt.want {
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

	result := checkModel(context.Background(), server.Client(), provider{BaseURL: server.URL}, "demo-model")

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

	result := checkModel(context.Background(), server.Client(), provider{BaseURL: server.URL}, "demo-model")

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

	result := checkModel(context.Background(), server.Client(), provider{BaseURL: server.URL}, "demo-model")

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

	result := checkModel(context.Background(), server.Client(), provider{BaseURL: server.URL}, "demo-model")

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
