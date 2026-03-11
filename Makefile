.PHONY: build run test test-integration lint clean tidy fmt docker-up docker-down docker-logs

APP_NAME := haru
BUILD_DIR := ./bin

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/server

run:
	DB_DRIVER=sqlite PORT=8080 go run ./cmd/server

test:
	go test ./... -v -count=1

test-integration:
	go test -v -count=1 ./tests/

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR) haru.db

tidy:
	go mod tidy

fmt:
	gofmt -w .

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f
