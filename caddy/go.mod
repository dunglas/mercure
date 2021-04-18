module github.com/dunglas/mercure/caddy

go 1.15

replace github.com/dunglas/mercure => ../

require (
	github.com/caddyserver/caddy/v2 v2.3.0
	github.com/dunglas/mercure v0.11.2
	github.com/klauspost/cpuid v1.3.1 // indirect
	github.com/libdns/libdns v0.2.0 // indirect
	github.com/mholt/acmez v0.1.3 // indirect
	github.com/miekg/dns v1.1.41 // indirect
	github.com/pelletier/go-toml v1.9.0 // indirect
	github.com/prometheus/client_golang v1.10.0
	github.com/stretchr/testify v1.7.0
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20210415154028-4f45737414dc // indirect
	golang.org/x/net v0.0.0-20210415231046-e915ea6b2b7d // indirect
	golang.org/x/sys v0.0.0-20210415045647-66c3f260301c // indirect
)
