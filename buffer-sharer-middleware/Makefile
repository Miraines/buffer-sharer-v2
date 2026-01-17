.PHONY: build build-linux run clean install

# Binary name
BINARY=buffer-sharer-middleware

# Build for current platform
build:
	go build -o $(BINARY) .

# Build for Linux (for VPS deployment)
build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY)-linux-amd64 .

# Build for Linux ARM (Raspberry Pi, etc.)
build-linux-arm:
	GOOS=linux GOARCH=arm64 go build -o $(BINARY)-linux-arm64 .

# Run locally
run:
	go run . -port 8080

# Clean build artifacts
clean:
	rm -f $(BINARY) $(BINARY)-linux-*

# Install on Linux server (run as root)
install: build-linux
	@echo "Installing Buffer Sharer Middleware..."
	sudo cp $(BINARY)-linux-amd64 /usr/local/bin/$(BINARY)
	sudo chmod +x /usr/local/bin/$(BINARY)
	sudo cp buffer-sharer-middleware.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable buffer-sharer-middleware
	@echo "Installation complete!"
	@echo "Start with: sudo systemctl start buffer-sharer-middleware"
	@echo "Check status: sudo systemctl status buffer-sharer-middleware"
	@echo "View logs: sudo journalctl -u buffer-sharer-middleware -f"
