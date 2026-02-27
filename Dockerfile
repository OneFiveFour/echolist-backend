# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o echolist-backend .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/echolist-backend .

# Create data and auth directories
RUN mkdir -p /app/data /app/auth

# Expose port
EXPOSE 8080

# Run the application
CMD ["./echolist-backend"]
