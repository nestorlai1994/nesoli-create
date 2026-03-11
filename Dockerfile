# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
ENV CGO_ENABLED=0 GOOS=linux

# Cache module downloads
COPY go.mod go.sum ./
RUN go mod download

# Build binary
COPY . .
RUN go build -ldflags="-s -w" -o nesoli-create .

# Runtime stage
FROM alpine:3.20
WORKDIR /app
RUN addgroup -S nesoli && adduser -S nesoli -G nesoli
COPY --from=builder /app/nesoli-create .
USER nesoli
EXPOSE 3000
CMD ["./nesoli-create"]
