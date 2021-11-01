module github.com/dunglas/mercure/caddy

go 1.16

replace github.com/dunglas/mercure => ../

require (
	github.com/caddyserver/caddy/v2 v2.4.3
	github.com/caddyserver/certmagic v0.14.1 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/dunglas/mercure v0.13.0
	github.com/google/uuid v1.3.0 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/miekg/dns v1.1.43 // indirect
	github.com/prometheus/client_golang v1.11.0
	github.com/smallstep/nosql v0.3.7 // indirect
	github.com/stretchr/testify v1.7.0
	go.uber.org/zap v1.19.0
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b // indirect
)
