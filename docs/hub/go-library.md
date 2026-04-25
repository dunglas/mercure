# Using Mercure as a Go Library

The recommended way to deploy Mercure is the prebuilt Caddy-embedded binary, the Docker image, or the Helm chart. All of them ship with a [Profile-Guided Optimization](https://go.dev/doc/pgo) (PGO) profile baked in, so no extra step is needed to benefit from it.

This page covers the advanced cases where you build the hub (or an application embedding it) from source, and want to keep the PGO speedup.

## Custom Caddy Build with xcaddy

When building a custom Caddy binary with [`xcaddy`](https://github.com/caddyserver/xcaddy), pass the profile that ships with the Mercure module through `XCADDY_GO_BUILD_FLAGS`:

```console
XCADDY_GO_BUILD_FLAGS="-pgo=$(go env GOMODCACHE)/github.com/dunglas/mercure@vX.Y.Z/default.pgo" \
    xcaddy build \
    --with github.com/dunglas/mercure/caddy
```

Replace `vX.Y.Z` with the Mercure version you want to build against.

## Embedding the Mercure Library in a Go Program

The Mercure hub can also be embedded directly in a Go program:

```go
package main

import "github.com/dunglas/mercure"

func main() {
    hub, err := mercure.NewHub( /* options */ )
    // ...
    _ = hub
}
```

Build your binary with the profile exposed by the Mercure module:

```console
go build -pgo="$(go list -m -f '{{.Dir}}' github.com/dunglas/mercure)/default.pgo" ./cmd/your-app
```

## Refreshing the Profile

The profile is regenerated on every release by `generate-pgo-profile.sh` at the repository root, which drives the [Gatling load test](load-test.md) against a local hub built from the current source. Run it manually to refresh the profile between releases, or to tailor it to a workload that differs from the default stress test.
