# Use the specific Golang version with Alpine to build the application
FROM golang:1.23.1-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go application
RUN go build -o /kcgen ./cmd/kcgen

# Use the same image for the final stage to avoid additional layers
FROM golang:1.23.1-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /kcgen .

# Define the entry point to accept command line arguments
ENTRYPOINT ["./kcgen"]
