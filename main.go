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

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
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
	Answer string `json:"answer,omitempty"`
	Error  string `json:"error,omitempty"`
}

func loadConfig() Config {
	return Config{
		OllamaModel:     getEnv("OLLAMA_MODEL", "llama3"),
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

var llm llms.LLM // Declare llm as a global variable
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections
	},
}

func processLLMRequest(query string, requestID string, conn *websocket.Conn) {
    maxRetries := 3
    retryDelay := 2 * time.Second
    log.Printf("RequestID: %s, Query: %s - Starting LLM request", requestID, query)

    for i := 0; i <= maxRetries; i++ {
        ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second) // Increased timeout
        defer cancel()

        var response string

        _, err := llm.Call(ctx, fmt.Sprintf("Human: %s\nAssistant:", query),
            llms.WithTemperature(0.8),
			llms.WithMaxTokens(100),
			llms.WithTopP(0.9),
			llms.WithFrequencyPenalty(0.0),
			llms.WithPresencePenalty(0.0),
            llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
                select {
				case <-ctx.Done():
                    log.Printf("RequestID: %s, Query: %s - Streaming context canceled: %v", requestID, query, ctx.Err())
                    err := conn.WriteJSON(Response{Error: "Stream Cancelled"})
					if err != nil {
						log.Printf("RequestID: %s, Query: %s - Error sending cancellation over websocket: %v", requestID, query, err)
					}
					return ctx.Err()

                default:
                    log.Printf("RequestID: %s, Query: %s - Chunk: %s", requestID, query, string(chunk))
					response += string(chunk)
                    err := conn.WriteJSON(Response{Answer: string(chunk)})
					if err != nil {
						log.Printf("RequestID: %s, Query: %s - Error sending chunk over websocket: %v", requestID, query, err)
                         if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway){
                            return err
                         }
                        return fmt.Errorf("error writing to websocket %w", err)
					}
                    return nil
                }
            }),
        )

        if err == nil {
            log.Printf("RequestID: %s, Query: %s - LLM request successful", requestID, query)
            err := conn.WriteJSON(Response{Answer: response})
            if err != nil {
                log.Printf("RequestID: %s, Query: %s - Error sending final response over websocket: %v", requestID, query, err)
                   if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway){
                     return
                   }
            }
			return
        }
        log.Printf("RequestID: %s, Query: %s - LLM request failed (attempt %d/%d): %v", requestID, query, i+1, maxRetries+1, err)
		if i < maxRetries {
			time.Sleep(retryDelay)
			retryDelay *= 2
		}
    }
	log.Printf("RequestID: %s, Query: %s - LLM request failed after %d retries", requestID, query, maxRetries)
    err := conn.WriteJSON(Response{Error: fmt.Sprintf("LLM request failed after %d retries", maxRetries)})
    if err != nil {
        log.Printf("RequestID: %s, Query: %s - Error sending final error over websocket: %v", requestID, query, err)
    }
}


func main() {
	config := loadConfig()
	rateLimiter := newRateLimiter(config.MaxRequests, config.RateLimitPeriod)


    var err error
	llm, err = ollama.New(
		ollama.WithModel(config.OllamaModel),
		ollama.WithServerURL(config.OllamaServerURL),
	)
	if err != nil {
		log.Fatalf("Failed to initialize LLM: %v", err)
	}

	sem := make(chan struct{}, config.MaxGoroutines)
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{http.MethodGet, http.MethodOptions, http.MethodPost},
		AllowedHeaders: []string{"Content-Type"},
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	})


	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {


		if !rateLimiter.Allow() {
			http.Error(w, `{"error":"Too many requests"}`, http.StatusTooManyRequests)
			return
		}

        conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Error upgrading connection to websocket: %v", err)
			return
		}
        defer conn.Close()

        requestID := uuid.New().String()

		sem <- struct{}{}
		defer func() { <-sem }()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
               if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
                  log.Printf("RequestID: %s, - WebSocket closed by client", requestID)
                  break
               }
               log.Printf("RequestID: %s, - Error reading message: %v", requestID, err)
               break
			}

			var req Request
			if err := json.Unmarshal(msg, &req); err != nil {
				log.Printf("RequestID: %s, - Error unmarshalling message: %v", requestID, err)
				err := conn.WriteJSON(Response{Error: "Invalid message format"})
				if err != nil {
					log.Printf("RequestID: %s - Error sending error over websocket: %v",requestID, err)
				}
				continue
			}

			if strings.TrimSpace(req.Query) == "" {
				log.Printf("RequestID: %s - Received empty query", requestID)
                 err := conn.WriteJSON(Response{Error: "Invalid query"})
				if err != nil {
					log.Printf("RequestID: %s - Error sending error over websocket: %v",requestID, err)
				}
				continue
			}

			log.Printf("RequestID: %s, Query from %s: %s", requestID, r.RemoteAddr, req.Query)
			processLLMRequest(req.Query, requestID, conn)
		}
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