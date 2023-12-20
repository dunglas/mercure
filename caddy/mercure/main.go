// Copied from https://github.com/caddyserver/xcaddy/blob/b7fd102f41e12be4735dc77b0391823989812ce8/environment.go#L251
package main

import (
	"github.com/caddyserver/caddy/v2"
	caddycmd "github.com/caddyserver/caddy/v2/cmd"

	"go.uber.org/automaxprocs/maxprocs"
	"go.uber.org/zap"

	// plug in Caddy modules here.
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	_ "github.com/dunglas/mercure/caddy"
)

func main() {
	undo, err := maxprocs.Set()
	defer undo()
	if err != nil {
		caddy.Log().Warn("failed to set GOMAXPROCS", zap.Error(err))
	}

	caddycmd.Main()
}
