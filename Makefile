# Binary name
BINARY_NAME=mastodon-scout
BINARY_LINUX=$(BINARY_NAME)-linux

# Build directory
BUILD_DIR=dist

# Linker flags for smaller binary size
LDFLAGS=-ldflags="-s -w"

.PHONY: build build-linux build-all clean test

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

build-linux:
	@echo "Building for Linux AMD64..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_LINUX) main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_LINUX)"

build-all: build build-linux

clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	@echo "Clean complete"

test:
	go test -v ./...
