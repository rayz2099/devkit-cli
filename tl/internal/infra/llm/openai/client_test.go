package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.internal.linran.top/linran/tl/internal/domain/translation"
)

func TestClientTranslateStreamsContentFromChatCompletions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q, want %q", request.URL.Path, "/chat/completions")
		}
		if got := request.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if got, _ := payload["model"].(string); got != "test-model" {
			t.Fatalf("model = %q, want %q", got, "test-model")
		}
		if got, _ := payload["stream"].(bool); !got {
			t.Fatal("stream = false, want true")
		}
		messages, ok := payload["messages"].([]any)
		if !ok {
			t.Fatalf("messages type = %T, want []any", payload["messages"])
		}
		if len(messages) != 2 {
			t.Fatalf("message count = %d, want 2", len(messages))
		}

		writer.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := writer.(http.Flusher)
		if !ok {
			t.Fatal("writer does not implement http.Flusher")
		}

		for _, chunk := range []string{
			`{"choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"thinking"},"finish_reason":null}]}`,
			`{"choices":[{"index":0,"delta":{"content":"{\"translation\":\"你"},"finish_reason":null}]}`,
			`{"choices":[{"index":0,"delta":{"content":"好\"}"},"finish_reason":null}]}`,
			`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		} {
			if _, err := fmt.Fprintf(writer, "data: %s\n\n", chunk); err != nil {
				t.Fatalf("Fprintf() error = %v", err)
			}
			flusher.Flush()
		}
		if _, err := fmt.Fprint(writer, "data: [DONE]\n\n"); err != nil {
			t.Fatalf("Fprint() error = %v", err)
		}
		flusher.Flush()
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL:      server.URL,
		Token:        "test-token",
		Model:        "test-model",
		CustomPrompt: "Keep punctuation.",
		HTTPClient:   server.Client(),
	})

	result, err := client.Translate(context.Background(), translation.DirectionEnToZh, "hello")
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	if result != "你好" {
		t.Fatalf("result = %q, want %q", result, "你好")
	}
}

func TestClientTranslateReturnsErrorWhenStreamHasNoContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := writer.(http.Flusher)
		if !ok {
			t.Fatal("writer does not implement http.Flusher")
		}

		for _, chunk := range []string{
			`{"choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"thinking"},"finish_reason":null}]}`,
			`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		} {
			if _, err := fmt.Fprintf(writer, "data: %s\n\n", chunk); err != nil {
				t.Fatalf("Fprintf() error = %v", err)
			}
			flusher.Flush()
		}
		if _, err := fmt.Fprint(writer, "data: [DONE]\n\n"); err != nil {
			t.Fatalf("Fprint() error = %v", err)
		}
		flusher.Flush()
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL:    server.URL,
		Token:      "test-token",
		Model:      "test-model",
		HTTPClient: server.Client(),
	})

	_, err := client.Translate(context.Background(), translation.DirectionEnToZh, "hello")
	if err == nil {
		t.Fatal("Translate() error = nil, want error")
	}
	if got := err.Error(); !strings.Contains(got, "streamed no content") {
		t.Fatalf("error = %q, want substring %q", got, "streamed no content")
	}
}
