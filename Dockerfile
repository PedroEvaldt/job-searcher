FROM golang:1.26.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/jobs_collector ./ \
    && CGO_ENABLED=0 GOOS=linux go build -o /app/radar ./cmd/radar

FROM alpine:3.23

WORKDIR /app
COPY --from=builder /app/config.yaml /app/jobs_collector /app/radar .
CMD ["./jobs_collector"]
