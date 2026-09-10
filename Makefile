# Routa Multi-Platform Build System

BINARY_NAME=routa
BUILD_DIR=bin
CMD_DIR=./cmd/routa

.PHONY: all clean build-all darwin-arm64 darwin-amd64 linux-arm64 linux-amd64 windows-amd64 windows-arm64

all: build-all

# Build all cross-platform targets
build-all: darwin-arm64 darwin-amd64 linux-arm64 linux-amd64 windows-amd64 windows-arm64

darwin-arm64:
	@echo "Building macOS ARM64 (Apple Silicon)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)

darwin-amd64:
	@echo "Building macOS AMD64 (Intel)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)

linux-arm64:
	@echo "Building Linux ARM64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)

linux-amd64:
	@echo "Building Linux AMD64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)

windows-amd64:
	@echo "Building Windows AMD64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)

windows-arm64:
	@echo "Building Windows ARM64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe $(CMD_DIR)

clean:
	rm -rf $(BUILD_DIR)
