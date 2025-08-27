# Use Go 1.22.1 as base image
FROM golang:1.22.1-alpine AS builder

# Set working directory inside the container
WORKDIR /app

# Copy go mod files first for better caching
COPY lola-ia/go.mod lola-ia/go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY lola-ia/ ./

# Build the application
RUN go build -o main .

# Use a minimal alpine image for the final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests (needed for OpenAI API)
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Expose port 8080 (default port used by the application)
EXPOSE 8080

# Command to run the application
CMD ["./main"]
