# Stage 1: Build the binary
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy module definition
COPY go.mod ./

# Copy all source code
COPY . .

# Tidy ensures go.mod and go.sum are synced and downloads dependencies.
# This is more robust than just 'go mod download'.
RUN go mod tidy

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Stage 2: Create the final small image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the pre-built binary from the first stage
COPY --from=builder /app/main .

# Expose port 8082 and start the app
EXPOSE 8082
CMD ["./main"]
