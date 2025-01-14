package main

import (
    "bufio"
    "context"
    "fmt"
    "log"
    "os"

    "github.com/tmc/langchaingo/llms"
    "github.com/tmc/langchaingo/llms/ollama"
)

func main() {
    // Initialize the LLM with the Ollama model
    llm, err := ollama.New(ollama.WithModel("llama3.2"))
    if err != nil {
        log.Fatalf("Failed to initialize LLM: %v", err)
    }

    ctx := context.Background()
    reader := bufio.NewReader(os.Stdin)

    fmt.Println("Enter your query (type 'exit' to quit):")
    for {
        fmt.Print("> ")
        userInput, err := reader.ReadString('\n')
        if err != nil {
            log.Fatalf("Error reading input: %v", err)
        }

        // Check for exit condition
        if userInput == "exit\n" {
            fmt.Println("Goodbye!")
            break
        }

        // Send input to the LLM and stream the response
        _, err = llm.Call(ctx, fmt.Sprintf("Human: %s\nAssistant:", userInput),
            llms.WithTemperature(0.8),
            llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
                fmt.Print(string(chunk))
                return nil
            }),
        )
        if err != nil {
            log.Printf("Error during LLM call: %v", err)
        }
        fmt.Println() // Add a newline after the response
    }
}
