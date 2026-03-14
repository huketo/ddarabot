APP_NAME := ddarabot
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: build run test lint fmt clean docker-build docker-deploy release

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

release:
	@for platform in $(PLATFORMS); do \
		OS=$${platform%/*}; \
		ARCH=$${platform#*/}; \
		echo "Building $$OS/$$ARCH..."; \
		CGO_ENABLED=0 GOOS=$$OS GOARCH=$$ARCH \
			go build -ldflags "-s -w -X main.version=$(VERSION)" \
			-o bin/$(APP_NAME)-$$OS-$$ARCH ./cmd/ddarabot/; \
	done
	@echo "Release binaries in bin/"
	@ls -lh bin/$(APP_NAME)-*

docker-build:
	docker build -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .

docker-deploy:
	docker compose up -d

clean:
	rm -rf bin/
