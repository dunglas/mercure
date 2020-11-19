module github.com/dunglas/mercure/caddy

go 1.15

replace github.com/dunglas/mercure => ../mercure

require (
	github.com/caddyserver/caddy/v2 v2.2.1
	github.com/dunglas/mercure v0.10.4
	github.com/gorilla/mux v1.8.0
	github.com/prometheus/client_golang v1.8.0
	github.com/stretchr/testify v1.5.1
	go.uber.org/zap v1.16.0
)
