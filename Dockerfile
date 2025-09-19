# syntax=docker/dockerfile:1
FROM docker.io/golang:1.24-alpine AS builder
WORKDIR /image
COPY . /image
WORKDIR /image/caddy
RUN go mod tidy && go build -o ../mercure mercure/main.go

FROM caddy:2-alpine
COPY --from=builder /image/mercure /usr/bin/caddy
RUN chmod +x /usr/bin/caddy