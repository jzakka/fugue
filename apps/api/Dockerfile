FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /fugue-api ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata ffmpeg

COPY --from=builder /fugue-api /usr/local/bin/fugue-api

ENTRYPOINT ["/usr/local/bin/fugue-api"]
