module github.com/dunglas/mercure

go 1.27

retract (
	v0.14.7 // CI problem
	v0.14.6 // Overwritten tag
)

require (
	github.com/dunglas/go-urlpattern v1.0.0
	github.com/dunglas/skipfilter v1.0.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/gorilla/mux v1.8.1
	github.com/maypok86/otter/v2 v2.3.0
	github.com/nlnwa/whatwg-url v0.6.2
	github.com/prometheus/client_golang v1.24.1
	github.com/prometheus/client_model v0.6.3
	github.com/rs/cors v1.11.1
	github.com/stretchr/testify v1.12.1
	github.com/unrolled/secure v1.17.0
	github.com/yosida95/uritemplate/v3 v3.0.2
	go.etcd.io/bbolt v1.5.0
	go.opentelemetry.io/otel v1.46.0
	go.opentelemetry.io/otel/sdk v1.44.0
	go.opentelemetry.io/otel/trace v1.46.0
)

require (
	github.com/MauriceGit/skiplist v0.0.0-20211105230623-77f5c8d3e145 // indirect
	github.com/RoaringBitmap/roaring/v2 v2.27.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.25.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mschoch/smat v0.2.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/common v0.71.0 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/exp v0.0.0-20260908205506-85c1c2202aba // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)
