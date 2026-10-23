# Scanner image for the Mercure.rocks Hub (main). threat_model.md, next to this file, says how to exercise it.
FROM golang:1.27-bookworm

# scanner contract: the checkout lives inside the image, at /src
COPY . /src
WORKDIR /src

# The tags release binaries and CI use.
ENV GOFLAGS=-tags=deprecated_transport,deprecated_topic,deprecated_claim,nobadger,nomysql,nopgx

RUN go mod download && (cd caddy && go mod download)

# The library, its tests (plain and -race, with and without the deprecated_topic and deprecated_claim tags), and the
# Caddy-based hub binary at /usr/local/bin/mercure. A step that fails fails the build.
RUN go build ./... \
    && go test -vet=off -run '^$' ./... \
    && go test -vet=off -race -run '^$' ./... \
    && GOFLAGS=-tags=deprecated_transport,nobadger,nomysql,nopgx go test -vet=off -run '^$' ./... \
    && cd caddy \
    && go test -vet=off -race -run '^$' ./... \
    && go build -o /usr/local/bin/mercure ./mercure

# Run the suites so the log shows the image works; a failing or hung test must not fail the build.
RUN (timeout 1500 go test -vet=off -race -count=1 -timeout 20m ./... || echo "WARNING: go test reported failures") \
    && (cd caddy && timeout 600 go test -vet=off -race -count=1 -timeout 5m ./... \
          || echo "WARNING: caddy go test reported failures")

# The audit is offline: no module proxy, no checksum database, no toolchain download.
ENV GOPROXY=off \
    GOSUMDB=off \
    GOTOOLCHAIN=local
