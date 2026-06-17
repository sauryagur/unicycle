FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# Download the Go module dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the Go application
RUN go build -o unicycle-server .

# Use a minimal base image
FROM alpine:latest

WORKDIR /root/

# Copy the built application from the builder stage
COPY --from=builder /app/unicycle-server .

# Expose the application port
EXPOSE 8080

# Run the application
CMD ["./unicycle-server"]

# Note: Customize the Dockerfile as needed for your specific project requirements.
# For example, you may need to add environment variables, additional dependencies, or other configurations.
