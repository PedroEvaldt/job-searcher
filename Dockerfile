# Go side of job-radar: one image, two binaries.
#   collector  pulls jobs from the sources into Postgres (compose services "app" and "migrate")
#   radar      the CLI: label, review, status, digest, digest-loop (compose service "digest")
FROM golang:1.26.1-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/collector ./cmd/collector \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/radar ./cmd/radar

FROM alpine:3.23

WORKDIR /app
# config.yaml is baked in as a default; compose mounts the real one over it.
COPY config.yaml ./
COPY --from=builder /out/collector /out/radar ./
CMD ["./collector"]
