# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/coin-alert ./cmd/server

# Run stage
FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /bin/coin-alert /bin/coin-alert
COPY templates ./templates
EXPOSE 5020
ENTRYPOINT ["/bin/coin-alert"]
