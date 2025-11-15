.PHONY: build test clean install

# Build the binary
build:
	go build -o splunk .

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f splunk

# Install to /usr/local/bin
install: build
	sudo cp splunk /usr/local/bin/splunk
	sudo chmod +x /usr/local/bin/splunk

# Build for all platforms
build-all:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o dist/splunk_darwin_amd64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o dist/splunk_darwin_arm64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -o dist/splunk_linux_386 .
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/splunk_linux_amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o dist/splunk_linux_arm64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/splunk_windows_amd64.exe .

# Run linter
lint:
	go vet ./...
	go fmt ./...

# Run the binary
run: build
	./splunk

# Show help
help:
	@echo "Available targets:"
	@echo "  build      - Build the splunk binary"
	@echo "  test       - Run tests"
	@echo "  clean      - Remove build artifacts"
	@echo "  install    - Install to /usr/local/bin"
	@echo "  build-all  - Build for all platforms"
	@echo "  lint       - Run go vet and go fmt"
	@echo "  run        - Build and run the binary"
	@echo "  help       - Show this help message"
