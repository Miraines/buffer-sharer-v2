.PHONY: all build build-darwin build-darwin-arm64 build-darwin-amd64 build-windows build-linux dev clean

# Default target
all: build

# Build for current platform
build:
	wails build

# Development mode
dev:
	wails dev

# Build for macOS (current architecture)
build-darwin:
	wails build -platform darwin

# Build for macOS Apple Silicon (M1/M2/M3)
build-darwin-arm64:
	wails build -platform darwin/arm64

# Build for macOS Intel
build-darwin-amd64:
	wails build -platform darwin/amd64

# Build for Windows (requires cross-compilation setup)
# On macOS you need: brew install mingw-w64
build-windows:
	wails build -platform windows/amd64

# Build for Linux (requires cross-compilation setup)
build-linux:
	wails build -platform linux/amd64

# Build all platforms
build-all: build-darwin-arm64 build-darwin-amd64 build-windows build-linux

# Clean build artifacts
clean:
	rm -rf build/bin/*

# Generate Wails bindings
generate:
	wails generate module

# Install frontend dependencies
frontend-install:
	cd frontend && npm install

# Run frontend dev server
frontend-dev:
	cd frontend && npm run dev

# Help
help:
	@echo "Buffer Sharer - Build Commands"
	@echo ""
	@echo "Usage:"
	@echo "  make build              - Build for current platform"
	@echo "  make dev                - Run in development mode"
	@echo "  make build-darwin       - Build for macOS (current arch)"
	@echo "  make build-darwin-arm64 - Build for macOS Apple Silicon"
	@echo "  make build-darwin-amd64 - Build for macOS Intel"
	@echo "  make build-windows      - Build for Windows (requires mingw-w64)"
	@echo "  make build-linux        - Build for Linux"
	@echo "  make build-all          - Build for all platforms"
	@echo "  make clean              - Clean build artifacts"
	@echo "  make generate           - Regenerate Wails bindings"
	@echo "  make help               - Show this help"
