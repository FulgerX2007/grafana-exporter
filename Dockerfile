FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files and download dependencies
COPY go.mod go.sum* ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o dashboard-exporter .

# Use a smaller image for the final container
FROM alpine:3.19

WORKDIR /app

# Copy the executable from the builder stage
COPY --from=builder /app/dashboard-exporter .

# Copy the public directory containing static files
COPY --from=builder /app/public ./public

# Create the export directory
RUN mkdir -p /app/exported

# Expose the port defined in the .env file (default: 8080)
EXPOSE 8080

# Run the application
CMD ["./dashboard-exporter"]