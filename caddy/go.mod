module github.com/dunglas/mercure/caddy

go 1.15

replace github.com/dunglas/mercure => ../

require (
	github.com/antlr/antlr4 v0.0.0-20210105212045-464bcbc32de2 // indirect
	github.com/caddyserver/caddy/v2 v2.3.0
	github.com/dunglas/mercure v0.11.0-rc.2
	github.com/prometheus/client_golang v1.9.0
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.16.0
	golang.org/x/sys v0.0.0-20210105210732-16f7687f5001 // indirect
	google.golang.org/genproto v0.0.0-20210106152847-07624b53cd92 // indirect
)
