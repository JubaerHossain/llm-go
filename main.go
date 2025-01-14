package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

// Config holds our application configuration
type Config struct {
	OllamaModel     string
	APIPort         string
	MaxRequests     int
	RateLimitPeriod time.Duration
}

// Request struct for incoming JSON payload
type Request struct {
	Query string `json:"query"`
}

// Response struct for JSON output
type Response struct {
	Answer string `json:"answer"`
}

// Initialize and return configurations
func loadConfig() Config {
	config := Config{
		OllamaModel:     getEnv("OLLAMA_MODEL", "llama3.2"),
		APIPort:         getEnv("API_PORT", "8080"),
		MaxRequests:     getEnvAsInt("MAX_REQUESTS", 10),
		RateLimitPeriod: getEnvAsDuration("RATE_LIMIT_PERIOD", 60*time.Second),
	}
	return config
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	var result int
	_, err := fmt.Sscan(value, &result)
	if err != nil {
		return defaultValue
	}
	return result
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return duration
}

type RateLimiter struct {
	mu              sync.Mutex
	lastRequest     time.Time
	requestCount    int
	maxRequests     int
	rateLimitPeriod time.Duration
}

func newRateLimiter(maxRequests int, rateLimitPeriod time.Duration) *RateLimiter {
	return &RateLimiter{
		lastRequest:     time.Now(),
		requestCount:    0,
		maxRequests:     maxRequests,
		rateLimitPeriod: rateLimitPeriod,
	}
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.Sub(rl.lastRequest) > rl.rateLimitPeriod {
		rl.requestCount = 0
	}
	if rl.requestCount < rl.maxRequests {
		rl.requestCount++
		rl.lastRequest = now
		return true
	}
	return false
}

// processLLMRequest handles the LLM processing
func processLLMRequest(llm llms.LLM, query string) (Response, error) {
	ctx := context.Background()
	var response string
	_, err := llm.Call(ctx, fmt.Sprintf("Human: %s\nAssistant:", query),
		llms.WithTemperature(0.8),
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			response += string(chunk)
			return nil
		}),
	)
	if err != nil {
		return Response{}, fmt.Errorf("error during LLM call: %v", err)
	}

	return Response{
		Answer: response,
	}, nil
}

func main() {
	// Load configurations
	config := loadConfig()

	// Initialize the rate limiter
	rateLimiter := newRateLimiter(config.MaxRequests, config.RateLimitPeriod)

	// Initialize the LLM with the Ollama model
	llm, err := ollama.New(ollama.WithModel(config.OllamaModel))
	if err != nil {
		log.Fatalf("Failed to initialize LLM: %v", err)
	}

	// Define the chat endpoint
	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		// Rate limiting
		if !rateLimiter.Allow() {
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}

		// Set response headers
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle OPTIONS request for CORS preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Only handle POST requests
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse request body
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Error decoding request body", http.StatusBadRequest)
			return
		}

		// Validate query
		if strings.TrimSpace(req.Query) == "" {
			http.Error(w, "Empty query", http.StatusBadRequest)
			return
		}

		// Log the received query
		log.Printf("Received query from %s: %s", r.RemoteAddr, req.Query)

		// Create channels for response and error
		responseChan := make(chan Response, 1)
		errorChan := make(chan error, 1)

		// Process LLM request in a goroutine
		go func() {
			resp, err := processLLMRequest(llm, req.Query)
			if err != nil {
				errorChan <- err
				return
			}
			responseChan <- resp
		}()

		// Wait for response or error with timeout
		select {
		case resp := <-responseChan:
			// Serialize and send the response
			respJson, err := json.Marshal(resp)
			if err != nil {
				http.Error(w, "Error creating response", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write(respJson)

		case err := <-errorChan:
			log.Println(err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		case <-time.After(30 * time.Second):
			http.Error(w, "Request timed out", http.StatusRequestTimeout)
		}
	})

	// Start the server
	serverAddr := fmt.Sprintf(":%s", config.APIPort)
	log.Printf("Server is running on port %s", config.APIPort)

	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}