.PHONY: build run test clean dev

BINARY=ricerkatoro
BUILD_DIR=./bin

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/ricerkatoro

run: build
	$(BUILD_DIR)/$(BINARY)

run-stdio: build
	TRANSPORT=stdio $(BUILD_DIR)/$(BINARY)

run-http: build
	TRANSPORT=http $(BUILD_DIR)/$(BINARY)

test:
	go test ./... -v

clean:
	rm -rf $(BUILD_DIR) ricerkatoro.db

tidy:
	go mod tidy

lint:
	golangci-lint run ./...

dev: tidy build
