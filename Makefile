.PHONY: build run test lint clean tidy fmt

APP_NAME := haru
BUILD_DIR := ./bin

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/server

run:
	DB_DRIVER=sqlite PORT=8080 go run ./cmd/server

test:
	go test ./... -v -count=1

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR) haru.db

tidy:
	go mod tidy

fmt:
	gofmt -w .
