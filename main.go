package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/rs/cors"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

type Config struct {
	OllamaModel     string
	OllamaServerURL string
	APIPort         string
	MaxRequests     int
	RateLimitPeriod time.Duration
	MaxGoroutines   int
}

type Request struct {
	Query string `json:"query"`
}

type Response struct {
	Answer string `json:"answer"`
	Error  string `json:"error,omitempty"`
}

func loadConfig() Config {
	return Config{
		OllamaModel:     getEnv("OLLAMA_MODEL", "llama3.2"),
		OllamaServerURL: getEnv("OLLAMA_SERVER_URL", "http://ollama:11434"),
		APIPort:         getEnv("API_PORT", "8080"),
		MaxRequests:     getEnvAsInt("MAX_REQUESTS", 10),
		RateLimitPeriod: getEnvAsDuration("RATE_LIMIT_PERIOD", 60*time.Second),
		MaxGoroutines:   getEnvAsInt("MAX_GOROUTINES", 100),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		if _, err := fmt.Sscan(value, &result); err == nil {
			return result
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

type RateLimiter struct {
	mu              sync.Mutex
	lastReset       time.Time
	requestCount    int
	maxRequests     int
	rateLimitPeriod time.Duration
}

func newRateLimiter(maxRequests int, rateLimitPeriod time.Duration) *RateLimiter {
	return &RateLimiter{
		lastReset:       time.Now(),
		maxRequests:     maxRequests,
		rateLimitPeriod: rateLimitPeriod,
	}
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.Sub(rl.lastReset) > rl.rateLimitPeriod {
		rl.requestCount = 0
		rl.lastReset = now
	}
	if rl.requestCount < rl.maxRequests {
		rl.requestCount++
		return true
	}
	return false
}

func processLLMRequest(llm llms.LLM, query string) (Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var response string
	_, err := llm.Call(ctx, fmt.Sprintf("Human: %s\nAssistant:", query),
		llms.WithTemperature(0.8),
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			response += string(chunk)
			return nil
		}),
	)
	if err != nil {
		return Response{}, fmt.Errorf("LLM error: %w", err)
	}

	return Response{Answer: response}, nil
}

func main() {
	config := loadConfig()
	rateLimiter := newRateLimiter(config.MaxRequests, config.RateLimitPeriod)

	llm, err := ollama.New(
		ollama.WithModel(config.OllamaModel),
		ollama.WithServerURL(config.OllamaServerURL),
	)
	if err != nil {
		log.Fatalf("Failed to initialize LLM: %v", err)
	}

	sem := make(chan struct{}, config.MaxGoroutines)
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{"Content-Type"},
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	})

	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if !rateLimiter.Allow() {
			http.Error(w, `{"error":"Too many requests"}`, http.StatusTooManyRequests)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"Only POST method is allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Query) == "" {
			http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
			return
		}

		log.Printf("Query from %s: %s", r.RemoteAddr, req.Query)

		sem <- struct{}{}
		defer func() { <-sem }()

		response, err := processLLMRequest(llm, req.Query)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
			return
		}

		respJson, _ := json.Marshal(response)
		w.WriteHeader(http.StatusOK)
		w.Write(respJson)
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", config.APIPort),
		Handler: c.Handler(mux),
	}

	go func() {
		log.Printf("Server running on port %s", config.APIPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("Shutting down server...")
	server.Shutdown(context.Background())
}
