---
title: "Distributed tracing for the Mercure hub with OpenTelemetry"
description: "Collect OpenTelemetry spans for Mercure publish, subscribe, and history operations by enabling Caddy's tracing directive and the standard OTEL_* variables."
---

# Tracing

The Mercure.rocks Hub emits [OpenTelemetry](https://opentelemetry.io/) spans for its main internal operations:

| Span name                   | Kind     | Description                                                                          |
| --------------------------- | -------- | ------------------------------------------------------------------------------------ |
| `mercure.publish`           | Producer | Covers the publish flow: authorization, validation, and dispatch to the transport    |
| `mercure.subscribe`         | Consumer | Covers subscription setup: authorization, history replay, and transport registration |
| `mercure.subscriptions`     | Internal | Covers requests to the subscription API (`/.well-known/mercure/subscriptions/...`)   |
| `mercure.transport.history` | Internal | Covers history replay from the storage transport (Bolt, Redis...)                    |

Mercure's spans nest under the HTTP request span produced by Caddy's [`tracing`](https://caddyserver.com/docs/caddyfile/directives/tracing) directive, so enable it to start collecting traces:

```caddyfile
# Caddyfile
route {
	tracing
	mercure {
		# ...
	}
}
```

Exporters, endpoints (OTLP gRPC or HTTP), protocols, resource attributes, and propagators are all configured through the standard [`OTEL_*` environment variables](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/).
When the `tracing` directive is not enabled, Mercure's spans are no-ops and have no runtime cost.

## Next steps

- [Health checks and monitoring](health-monitoring.md) for Prometheus metrics and health endpoints.
- [Debugging](debugging.md) for the structured logs that complement traces.
