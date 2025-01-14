# Makefile for LLM API Project

# Project variables
PROJECT_NAME := llm-api
VERSION := $(shell git describe --tags --always --dirty)
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION := $(shell go version | cut -d' ' -f3)

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOCLEAN := $(GOCMD) clean

# Docker parameters
DOCKER := docker
DOCKER_COMPOSE := docker-compose
DOCKER_TAG := $(VERSION)

# Directories
SRC_DIR := .
BUILD_DIR := ./build
BINARY_NAME := llm-api

# Go build flags
LDFLAGS := -ldflags "-X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)'"

# Color output
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m

# Default target
.PHONY: all
all: clean deps test build

# Clean build artifacts
.PHONY: clean
clean:
	@echo "${GREEN}Cleaning build artifacts...${NC}"
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

# Install dependencies
.PHONY: deps
deps:
	@echo "${GREEN}Installing dependencies...${NC}"
	$(GOMOD) download
	$(GOMOD) tidy

# Run tests
.PHONY: test
test:
	@echo "${GREEN}Running tests...${NC}"
	$(GOTEST) -v ./...
	$(GOTEST) -cover -coverprofile=coverage.out

# Run tests with race detector
.PHONY: test-race
test-race:
	@echo "${GREEN}Running tests with race detector...${NC}"
	$(GOTEST) -race ./...

# Build the application
.PHONY: build
build:
	@echo "${GREEN}Building $(PROJECT_NAME) version $(VERSION)...${NC}"
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(SRC_DIR)

# Cross-compilation
.PHONY: build-all
build-all:
	@echo "${GREEN}Cross-compiling for multiple platforms...${NC}"
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(SRC_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(SRC_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(SRC_DIR)

# Docker build
.PHONY: docker-build
docker-build:
	@echo "${GREEN}Building Docker image $(PROJECT_NAME):$(DOCKER_TAG)...${NC}"
	$(DOCKER) build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(PROJECT_NAME):$(DOCKER_TAG) .

# Docker compose commands
.PHONY: up
up:
	@echo "${GREEN}Starting services with Docker Compose...${NC}"
	$(DOCKER_COMPOSE) up --build -d

.PHONY: down
down:
	@echo "${GREEN}Stopping services...${NC}"
	$(DOCKER_COMPOSE) down

.PHONY: logs
logs:
	@echo "${GREEN}Showing service logs...${NC}"
	$(DOCKER_COMPOSE) logs -f

# Ollama model management
.PHONY: pull-model
pull-model:
	@echo "${GREEN}Pulling Ollama model...${NC}"
	$(DOCKER) exec -it ollama ollama pull llama3

.PHONY: list-models
list-models:
	@echo "${GREEN}Listing Ollama models...${NC}"
	$(DOCKER) exec -it ollama ollama list

# Development run
.PHONY: dev
dev:
	@echo "${GREEN}Running in development mode...${NC}"
	$(GOCMD) run main.go

# Code quality and security
.PHONY: lint
lint:
	@echo "${GREEN}Running golangci-lint...${NC}"
	golangci-lint run ./...

.PHONY: sec-scan
sec-scan:
	@echo "${GREEN}Running security scan...${NC}"
	gosec ./...

# Generate documentation
.PHONY: docs
docs:
	@echo "${GREEN}Generating API documentation...${NC}"
	swag init

# Show project information
.PHONY: info
info:
	@echo "${GREEN}Project: $(PROJECT_NAME)${NC}"
	@echo "${YELLOW}Version: $(VERSION)${NC}"
	@echo "${YELLOW}Go Version: $(GO_VERSION)${NC}"
	@echo "${YELLOW}Build Time: $(BUILD_TIME)${NC}"

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all        - Clean, install deps, run tests, and build"
	@echo "  clean      - Remove build artifacts"
	@echo "  deps       - Install dependencies"
	@echo "  test       - Run tests"
	@echo "  build      - Build the application"
	@echo "  docker-build - Build Docker image"
	@echo "  up         - Start services with Docker Compose"
	@echo "  down       - Stop services"
	@echo "  dev        - Run in development mode"
	@echo "  lint       - Run linter"
	@echo "  sec-scan   - Run security scan"
	@echo "  info       - Show project information"