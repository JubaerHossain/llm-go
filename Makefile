# Define variables for the image and container name
OLLAMA_IMAGE = ollama/ollama:latest
OLLAMA_CONTAINER = ollama
GO_APP = ./main.go
API_PORT = 8080
OLLAMA_PORT = 11411

# Default target to run everything
all: build run-ollama start-server

# Build the Go application
build:
	@echo "Building the Go application..."
	go build -o myapp $(GO_APP)

# Build and run the Ollama Docker container
run-ollama:
	@echo "Building and running Ollama Docker container..."
	docker pull $(OLLAMA_IMAGE)
	docker run -d -p $(OLLAMA_PORT):$(OLLAMA_PORT) --name $(OLLAMA_CONTAINER) $(OLLAMA_IMAGE)
	@echo "Ollama Docker container is running on port $(OLLAMA_PORT)"

# Start the Go server
start-server:
	@echo "Starting the Go server..."
	./myapp

# Stop the Ollama Docker container
stop-ollama:
	@echo "Stopping the Ollama Docker container..."
	docker stop $(OLLAMA_CONTAINER)
	docker rm $(OLLAMA_CONTAINER)

# Clean up build artifacts and stop containers
clean: stop-ollama
	@echo "Cleaning up..."
	rm -f myapp

# Display help for using the Makefile
help:
	@echo "Makefile Commands:"
	@echo "  make all             Build Go app, start Ollama container, and start the server"
	@echo "  make build           Build the Go app"
	@echo "  make run-ollama      Pull and run the Ollama Docker container"
	@echo "  make start-server    Start the Go server"
	@echo "  make stop-ollama     Stop the Ollama Docker container"
	@echo "  make clean           Clean up build artifacts and stop Ollama container"
