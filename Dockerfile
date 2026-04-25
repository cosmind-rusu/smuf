# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o smuf-server ./cmd/smuf-server

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /app/smuf-server /app/smuf-server

EXPOSE 7000 8080 443 80

ENTRYPOINT ["/app/smuf-server"]
