// Copied from https://github.com/caddyserver/xcaddy/blob/b7fd102f41e12be4735dc77b0391823989812ce8/environment.go#L251
package main

import (
	"log/slog"

	"github.com/caddyserver/caddy/v2"

	"github.com/KimMachineGun/automemlimit/memlimit"
	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	"go.uber.org/zap/exp/zapslog"

	// plug in Caddy modules here.
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	_ "github.com/dunglas/mercure/caddy"
	_ "go.uber.org/automaxprocs"
)

func main() {
	// Backport of https://github.com/caddyserver/caddy/pull/6809
	// TODO: remove this block when Caddy 2.10 will be released
	_, _ = memlimit.SetGoMemLimitWithOpts(
		memlimit.WithLogger(
			slog.New(zapslog.NewHandler(caddy.Log().Core())),
		),
		memlimit.WithProvider(
			memlimit.ApplyFallback(
				memlimit.FromCgroup,
				memlimit.FromSystem,
			),
		),
	)

	caddycmd.Main()
}
