APP_NAME := ddarabot
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build run test lint fmt clean

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(APP_NAME) ./cmd/ddarabot/

run:
	go run ./cmd/ddarabot/ --config config.toml

test:
	go test ./... -v

lint:
	@test -z "$$(gofmt -l ./cmd/ ./internal/)" || (gofmt -l ./cmd/ ./internal/ && echo "Run 'make fmt' to fix" && exit 1)
	go vet ./...

fmt:
	gofmt -w ./cmd/ ./internal/

clean:
	rm -rf bin/
