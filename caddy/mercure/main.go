// Adapted from https://github.com/caddyserver/xcaddy/blob/b7fd102f41e12be4735dc77b0391823989812ce8/environment.go#L251
package main

import (
	"fmt"
	"runtime/debug"

	caddy "github.com/caddyserver/caddy/v2"
	caddycmd "github.com/caddyserver/caddy/v2/cmd"

	// plug in Caddy modules here.
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	_ "github.com/dunglas/mercure/caddy"
)

//nolint:gochecknoinits
func init() {
	if caddy.CustomVersion != "" {
		return
	}

	version := "(unknown)"
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, m := range bi.Deps {
			if m.Path == "github.com/dunglas/mercure" {
				version = m.Version

				break
			}
		}
	}

	caddy.CustomVersion = fmt.Sprintf("Mercure.rocks %s Caddy", version)
}

func main() {
	caddycmd.Main()
}
