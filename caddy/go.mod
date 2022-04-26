module github.com/dunglas/mercure/caddy

go 1.16

replace github.com/dunglas/mercure => ../

require (
	github.com/caddyserver/caddy/v2 v2.5.0
	github.com/dunglas/mercure v0.13.0
	github.com/klauspost/cpuid/v2 v2.0.12 // indirect
	github.com/lucas-clemente/quic-go v0.26.0 // indirect
	github.com/miekg/dns v1.1.48 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/prometheus/client_golang v1.12.1
	github.com/stretchr/testify v1.7.1
	go.uber.org/zap v1.21.0
	golang.org/x/term v0.0.0-20220411215600-e5f449aeb171 // indirect
	golang.org/x/tools v0.1.10 // indirect
)
