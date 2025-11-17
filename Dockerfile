# Multi-stage Dockerfile for JASM (Just Another Secret Manager)
# Supports multi-architecture builds

# Stage 1: Build
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY internal/ internal/

# Build the binary for target architecture
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-w -s" -o controller ./cmd/controller/main.go

# Stage 2: Runtime
FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /

# Copy the binary from builder (ensuring it's executable)
COPY --from=builder --chmod=755 /workspace/controller /controller

# Use nonroot user
USER 65532:65532

# Set entrypoint
ENTRYPOINT ["/controller"]
