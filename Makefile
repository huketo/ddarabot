APP_NAME := ddarabot
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build run test lint fmt clean docker-build docker-deploy

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

docker-build:
	docker build -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .

docker-deploy:
	docker compose up -d

clean:
	rm -rf bin/
