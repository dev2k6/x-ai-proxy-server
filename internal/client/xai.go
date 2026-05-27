package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"x-ai-proxy-server/internal/models"
)

type XAIClient struct {
	client  *http.Client
	baseURL string
	headers map[string]string
}

func NewXAIClient(baseURL string, headers map[string]string) *XAIClient {
	return &XAIClient{
		client: &http.Client{
			Transport: &http.Transport{
				ForceAttemptHTTP2:    true,
				MaxIdleConns:         1,
				IdleConnTimeout:      90 * time.Second,
				TLSHandshakeTimeout:  30 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
			Timeout: 120 * time.Second,
		},
		baseURL: baseURL,
		headers: headers,
	}
}

func (c *XAIClient) SendRequest(ctx context.Context, req *models.XAIRequest) (*http.Response, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/responses", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	for k, v := range c.headers {
		httpReq.Header.Add(k, v)
	}

	return c.client.Do(httpReq)
}

func (c *XAIClient) ParseSSEStream(body io.Reader, onDelta func(string)) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataStr := strings.TrimPrefix(line, "data: ")
			if dataStr == "[DONE]" {
				continue
			}
			var event models.XAIEvent
			if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
				continue
			}
			if event.Type == "response.output_text.delta" && event.Delta != "" {
				onDelta(event.Delta)
			}
		}
	}
	return scanner.Err()
}

func (c *XAIClient) CollectFullText(body io.Reader) (string, error) {
	var fullText strings.Builder
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataStr := strings.TrimPrefix(line, "data: ")
			if dataStr == "[DONE]" {
				continue
			}
			var event models.XAIEvent
			if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
				continue
			}
			if event.Type == "response.output_text.delta" && event.Delta != "" {
				fullText.WriteString(event.Delta)
			}
		}
	}
	return fullText.String(), scanner.Err()
}

func BuildXAIRequest(req *models.OpenAIRequest) *models.XAIRequest {
	xaiInput := models.XAIInput{Role: "user", Content: []models.XAIContent{}}
	for _, msg := range req.Messages {
		xaiInput.Content = append(xaiInput.Content, models.XAIContent{Type: "input_text", Text: msg.Content.String()})
	}

	temp := 0.7
	if req.Temperature != nil {
		temp = *req.Temperature
	}
	topP := 0.95
	if req.TopP != nil {
		topP = *req.TopP
	}
	maxTokens := 1000000
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	return &models.XAIRequest{
		Model:           req.Model,
		Input:           []models.XAIInput{xaiInput},
		MaxOutputTokens: maxTokens,
		Temperature:     temp,
		TopP:            topP,
		Reasoning:       models.XAIReasoning{Effort: "low"},
		Store:           false,
		Include:         []string{"reasoning.encrypted_content"},
		Tools: []models.XAITool{
			{Type: "web_search", EnableImageUnderstanding: true},
			{Type: "x_search", EnableVideoUnderstanding: true},
		},
		ToolChoice: "auto",
		Stream:     true,
	}
}