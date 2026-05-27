package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"x-ai-proxy-server/internal/client"
	"x-ai-proxy-server/internal/handlers"
)

type Config struct {
	Host    string            `json:"host"`
	Port    string            `json:"port"`
	ApiKey  string            `json:"api_key"`
	Headers map[string]string `json:"headers"`
}

func loadConfig(path string) Config {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal("config.json not found: ", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatal("invalid config.json: ", err)
	}
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == "" {
		cfg.Port = "60443"
	}
	return cfg
}

func requireAuth(apiKey string, next http.HandlerFunc) http.HandlerFunc {
	if apiKey == "" {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + apiKey
		if auth != expected {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": "Invalid API key",
					"type":    "invalid_request_error",
					"code":    "invalid_api_key",
				},
			})
			return
		}
		next(w, r)
	}
}

func main() {
	cfg := loadConfig("config.json")

	xaiClient := client.NewXAIClient("https://console.x.ai", cfg.Headers)
	chatHandler := handlers.NewChatHandler(xaiClient)

	http.HandleFunc("/v1/chat/completions", requireAuth(cfg.ApiKey, chatHandler.HandleChatCompletions))
	http.HandleFunc("/v1/models", requireAuth(cfg.ApiKey, chatHandler.HandleModels))

	addr := cfg.Host + ":" + cfg.Port
	log.Println("OpenAI-compatible proxy listening on " + addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}