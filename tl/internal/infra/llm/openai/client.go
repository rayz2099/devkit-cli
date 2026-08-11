package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"git.internal.linran.top/linran/tl/internal/domain/translation"
)

type ClientConfig struct {
	BaseURL      string
	Token        string
	Model        string
	CustomPrompt string
	HTTPClient   *http.Client
}

type Client struct {
	baseURL      string
	token        string
	model        string
	customPrompt string
	httpClient   *http.Client
}

type chatCompletionRequest struct {
	Model          string         `json:"model"`
	Messages       []chatMessage  `json:"messages"`
	ResponseFormat responseFormat `json:"response_format,omitempty"`
	Stream         bool           `json:"stream,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatCompletionChunk struct {
	Choices []chatCompletionChunkChoice `json:"choices"`
}

type chatCompletionChunkChoice struct {
	Delta        chatCompletionDelta `json:"delta"`
	FinishReason string              `json:"finish_reason"`
}

type chatCompletionDelta struct {
	Content *string `json:"content"`
}

type translationPayload struct {
	Direction   string `json:"direction"`
	Text        string `json:"text,omitempty"`
	Translation string `json:"translation,omitempty"`
}

func NewClient(cfg ClientConfig) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL:      strings.TrimRight(cfg.BaseURL, "/"),
		token:        cfg.Token,
		model:        cfg.Model,
		customPrompt: cfg.CustomPrompt,
		httpClient:   httpClient,
	}
}

func (c *Client) Translate(ctx context.Context, direction translation.Direction, text string) (string, error) {
	payload := translationPayload{
		Direction: string(direction),
		Text:      text,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	requestBody, err := json.Marshal(chatCompletionRequest{
		Model: c.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: buildSystemPrompt(direction, c.customPrompt),
			},
			{
				Role:    "user",
				Content: string(body),
			},
		},
		ResponseFormat: responseFormat{Type: "json_object"},
		Stream:         true,
	})
	if err != nil {
		return "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(requestBody))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("openai provider returned status %d", response.StatusCode)
	}

	content, err := readStreamContent(response.Body)
	if err != nil {
		return "", err
	}

	var translated translationPayload
	if err := json.Unmarshal([]byte(content), &translated); err != nil {
		return "", fmt.Errorf("openai provider returned invalid streamed JSON: %w", err)
	}
	if translated.Translation == "" {
		return "", fmt.Errorf("openai provider returned empty translation")
	}
	return translated.Translation, nil
}

func buildSystemPrompt(direction translation.Direction, customPrompt string) string {
	base := "Translate the input Markdown fragment and return JSON object {\"translation\": \"...\"}. Preserve Markdown syntax, headings, list markers, URLs, and code spans."
	if direction == translation.DirectionZhToEn {
		base = "Translate the input Markdown fragment from Chinese into English and return JSON object {\"translation\": \"...\"}. Preserve Markdown syntax, headings, list markers, URLs, and code spans."
	}
	if customPrompt == "" {
		return base
	}
	return base + " " + customPrompt
}

func readStreamContent(body io.Reader) (string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var builder strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if data == "[DONE]" {
			break
		}

		var chunk chatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return "", err
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != nil {
				builder.WriteString(*choice.Delta.Content)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	content := strings.TrimSpace(builder.String())
	if content == "" {
		return "", fmt.Errorf("openai provider streamed no content")
	}
	return content, nil
}
