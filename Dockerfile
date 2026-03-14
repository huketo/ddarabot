FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o ddarabot ./cmd/ddarabot/

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/ddarabot .
VOLUME ["/app/data"]
ENTRYPOINT ["./ddarabot"]
CMD ["--config", "/app/data/config.toml"]
