package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"x-ai-proxy-server/internal/client"
	"x-ai-proxy-server/internal/models"
)

type ChatHandler struct {
	xaiClient *client.XAIClient
}

func NewChatHandler(xaiClient *client.XAIClient) *ChatHandler {
	return &ChatHandler{xaiClient: xaiClient}
}

func (h *ChatHandler) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		writeOpenAIError(w, "Method not allowed", "invalid_request_error", "", "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}

	var openaiReq models.OpenAIRequest
	if err := json.NewDecoder(r.Body).Decode(&openaiReq); err != nil {
		writeOpenAIError(w, err.Error(), "invalid_request_error", "", "invalid_json", http.StatusBadRequest)
		return
	}

	xaiReq := client.BuildXAIRequest(&openaiReq)
	resp, err := h.xaiClient.SendRequest(context.Background(), xaiReq)
	if err != nil {
		writeOpenAIError(w, err.Error(), "api_error", "", "request_failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		writeOpenAIError(w, fmt.Sprintf("xAI API error: %s", string(body)), "api_error", "", "upstream_error", http.StatusBadGateway)
		return
	}

	if openaiReq.Stream {
		h.streamResponse(w, resp.Body)
	} else {
		h.collectAndRespond(w, resp.Body, openaiReq.Model)
	}
}

func writeOpenAIError(w http.ResponseWriter, message, errorType, param, code string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	errResp := models.OpenAIError{}
	errResp.Error.Message = message
	errResp.Error.Type = errorType
	errResp.Error.Param = param
	errResp.Error.Code = code
	json.NewEncoder(w).Encode(errResp)
}

func (h *ChatHandler) streamResponse(w http.ResponseWriter, body io.Reader) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeOpenAIError(w, "Streaming not supported", "server_error", "", "streaming_unsupported", http.StatusInternalServerError)
		return
	}

	finishReason := "stop"
	h.xaiClient.ParseSSEStream(body, func(delta string) {
		chunk := models.OpenAIChunk{
			ID:      "chatcmpl-" + fmt.Sprint(time.Now().UnixNano()),
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "grok-4.3",
			Choices: []struct {
				Index int `json:"index"`
				Delta struct {
					Role    string `json:"role,omitempty"`
					Content string `json:"content,omitempty"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Delta: struct {
						Role    string `json:"role,omitempty"`
						Content string `json:"content,omitempty"`
					}{Content: delta},
				},
			},
		}
		out, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", out)
		flusher.Flush()
	})

	// Final chunk with finish_reason
	finalChunk := models.OpenAIChunk{
		ID:      "chatcmpl-" + fmt.Sprint(time.Now().UnixNano()),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   "grok-4.3",
		Choices: []struct {
			Index int `json:"index"`
			Delta struct {
				Role    string `json:"role,omitempty"`
				Content string `json:"content,omitempty"`
			} `json:"delta"`
			FinishReason *string `json:"finish_reason"`
		}{
			{
				Index:        0,
				Delta:        struct {
					Role    string `json:"role,omitempty"`
					Content string `json:"content,omitempty"`
				}{},
				FinishReason: &finishReason,
			},
		},
	}
	out, _ := json.Marshal(finalChunk)
	fmt.Fprintf(w, "data: %s\n\n", out)
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (h *ChatHandler) collectAndRespond(w http.ResponseWriter, body io.Reader, model string) {
	fullText, _ := h.xaiClient.CollectFullText(body)

	openaiResp := models.OpenAIResponse{
		ID:      "chatcmpl-" + fmt.Sprint(time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			{
				Index: 0,
				Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{
					Role:    "assistant",
					Content: fullText,
				},
				FinishReason: "stop",
			},
		},
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{0, 0, 0},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(openaiResp)
}

func (h *ChatHandler) HandleModels(w http.ResponseWriter, r *http.Request) {
	now := time.Now().Unix()
	modelsList := map[string]any{
		"object": "list",
		"data": []map[string]any{
			{"id": "grok-build-0.1", "object": "model", "created": now, "owned_by": "x.ai"},
			{"id": "grok-4.3", "object": "model", "created": now, "owned_by": "x.ai"},
			{"id": "grok-4.20-multi-agent-0309", "object": "model", "created": now, "owned_by": "x.ai"},
			{"id": "grok-4.20-0309-reasoning", "object": "model", "created": now, "owned_by": "x.ai"},
			{"id": "grok-4.20-0309-non-reasoning", "object": "model", "created": now, "owned_by": "x.ai"},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(modelsList)
}