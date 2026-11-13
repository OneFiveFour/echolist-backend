# Build stage — runs natively on the build host's arch, cross-compiles to the target arch
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

# Populated automatically by BuildKit
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application (cross-compile to the requested target platform; no QEMU needed)
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o echolist-backend .

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
