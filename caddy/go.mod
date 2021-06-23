module github.com/dunglas/mercure/caddy

go 1.16

replace github.com/dunglas/mercure => ../

require (
	github.com/caddyserver/caddy/v2 v2.3.0
	github.com/dunglas/mercure v0.11.3
	github.com/prometheus/client_golang v1.10.0
	github.com/stretchr/testify v1.7.0
	go.uber.org/zap v1.16.0
)
