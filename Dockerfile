FROM golang AS builder

WORKDIR /src
# Download dependencies
COPY go.mod go.sum /
RUN go mod download

# Add source code
COPY . .
# delete any *.log files
RUN find . -name "*.log" -type f -delete
RUN CGO_ENABLED=0 go build -o main .

# Multi-Stage production build
FROM alpine AS production
RUN apk --no-cache add ca-certificates

WORKDIR /app
# Retrieve the binary from the previous stage
COPY --from=builder /src/main .

# Expose port
EXPOSE 3000
# Set the binary as the entrypoint of the container
CMD ["./main", "serve"]