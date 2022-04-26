module github.com/dunglas/mercure/caddy

go 1.16

replace github.com/dunglas/mercure => ../

require (
	github.com/caddyserver/caddy/v2 v2.4.3
	github.com/caddyserver/certmagic v0.14.1 // indirect
	github.com/dunglas/mercure v0.13.0
	github.com/fsnotify/fsnotify v1.5.3 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/lucas-clemente/quic-go v0.27.0 // indirect
	github.com/miekg/dns v1.1.43 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/prometheus/client_golang v1.12.1
	github.com/smallstep/nosql v0.3.7 // indirect
	github.com/stretchr/testify v1.7.1
	go.uber.org/zap v1.21.0
	golang.org/x/crypto v0.0.0-20220411220226-7b82a4e95df4 // indirect
	golang.org/x/net v0.0.0-20220425223048-2871e0cb64e4 // indirect
	golang.org/x/sys v0.0.0-20220422013727-9388b58f7150 // indirect
	golang.org/x/tools v0.1.10 // indirect
	golang.org/x/xerrors v0.0.0-20220411194840-2f41105eb62f // indirect
)
