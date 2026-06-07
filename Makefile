.PHONY: build test test-cover lint clean run fmt vet tidy check

# Build the golem binary
build:
	go build -o bin/golem ./cmd/golem

# Run all tests
test:
	go test -v ./...

# Run tests with coverage
test-cover:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	golangci-lint run

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Run the application
run:
	go run ./cmd/golem

# Format code
fmt:
	gofmt -s -w .

# Run go vet
vet:
	go vet ./...

# Tidy dependencies
tidy:
	go mod tidy

# Run all checks (fmt, vet, test)
check: fmt vet test
