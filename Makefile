.PHONY: build run clean install deps fmt lint test

BINARY_NAME=messages-tui
BUILD_DIR=./build
CMD_DIR=./cmd/messages-tui

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

# Run the application
run: build
	@$(BUILD_DIR)/$(BINARY_NAME)

# Run without building (for development)
dev:
	go run $(CMD_DIR)/main.go

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	go clean

# Install to $GOPATH/bin
install:
	go install $(CMD_DIR)

# Download dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run

# Run tests
test:
	go test -v ./...

# Build for multiple platforms
build-all: clean
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)
