APP_NAME := ddarabot
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build run test clean

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(APP_NAME) ./cmd/ddarabot/

run:
	go run ./cmd/ddarabot/ --config config.toml

test:
	go test ./... -v

clean:
	rm -rf bin/
