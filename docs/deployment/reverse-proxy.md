---
title: "Run Mercure Behind NGINX, Traefik, Caddy, HAProxy, or Cloudflare"
description: "Reverse-proxy the Mercure.rocks Hub behind NGINX, Traefik, Caddy, HAProxy, AWS ALB, or Cloudflare with SSE-friendly buffering and timeouts."
---

# Reverse Proxies

Mercure works behind any HTTP reverse proxy that can keep a streaming response open. Two configurations matter, and most defaults get them wrong:

1. **Don't buffer the response.** SSE pushes events as they're written; buffering holds them until the buffer fills, which delays everything by seconds.
2. **Long read timeouts.** A typical 30s or 60s read timeout closes every SSE connection that goes idle.

Below are working configurations for the proxies people use most. Adapt for your setup.

## NGINX

```nginx
# NGINX
server {
    listen 443 ssl http2;
    server_name hub.example.com;

    ssl_certificate     /etc/ssl/hub.example.com.crt;
    ssl_certificate_key /etc/ssl/hub.example.com.key;

    location / {
        proxy_pass http://mercure-upstream;
        proxy_http_version 1.1;

        # Don't buffer the response — SSE relies on immediate flush
        proxy_buffering off;
        proxy_cache off;

        # Long read timeout — SSE connections live for hours
        proxy_read_timeout 24h;

        # Forwarded headers (only enable USE_FORWARDED_HEADERS=1 on the hub
        # if NGINX is the only thing in front and these are sanitized)
        proxy_set_header Connection "";
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Host  $host;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

upstream mercure-upstream {
    server 127.0.0.1:8080;
}
```

Key directives:

- `proxy_buffering off` — without this, NGINX may hold events for seconds until the buffer fills. The single most common cause of "events arrive in batches."
- `proxy_read_timeout 24h` — NGINX's default is 60 seconds. Anything less than your `heartbeat` setting (40s default on the hub) will drop connections regularly.
- `proxy_http_version 1.1` and `Connection ""` — tell NGINX not to add a `Connection: close` and not to downgrade from HTTP/2 between the client and itself.

## Traefik

Traefik is well-behaved out of the box for SSE — no buffering, sensible timeouts. A working `compose.yaml`:

```yaml
# Traefik
services:
  reverse-proxy:
    image: traefik:v3
    command:
      - "--providers.docker"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.le.acme.email=ops@example.com"
      - "--certificatesresolvers.le.acme.storage=/letsencrypt/acme.json"
      - "--certificatesresolvers.le.acme.httpchallenge.entrypoint=web"
    ports: ["80:80", "443:443"]
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - traefik_letsencrypt:/letsencrypt

  mercure:
    image: dunglas/mercure
    restart: unless-stopped
    environment:
      SERVER_NAME: ":80"  # let Traefik handle TLS
      MERCURE_PUBLISHER_JWT_KEY: "!ChangeThisMercureHubJWTSecretKey!"
      MERCURE_SUBSCRIBER_JWT_KEY: "!ChangeThisMercureHubJWTSecretKey!"
    volumes:
      - mercure_data:/data
      - mercure_config:/config
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.mercure.rule=Host(`hub.example.com`)"
      - "traefik.http.routers.mercure.entrypoints=websecure"
      - "traefik.http.routers.mercure.tls.certresolver=le"
      - "traefik.http.services.mercure.loadbalancer.server.port=80"

volumes:
  traefik_letsencrypt:
  mercure_data:
  mercure_config:
```

Disable the hub's own TLS (`SERVER_NAME=:80`) — Traefik handles it.

For long write timeouts on Traefik (the default per-router timeout is generous, but worth pinning):

```yaml
# Traefik
labels:
  - "traefik.http.services.mercure.loadbalancer.responseforwarding.flushinterval=1ms"
```

`flushinterval` controls how often Traefik flushes streaming responses to the client. The default is fine for most cases; lower it if you observe events queuing.

## Caddy

If you're terminating TLS in another Caddy instance (or fronting Mercure with a separate Caddy reverse proxy):

```caddyfile
# Caddy
hub.example.com {
  reverse_proxy mercure:80 {
    flush_interval -1     # flush every write
    transport http {
      versions 1.1 2
      read_timeout 24h
      response_header_timeout 24h
    }
  }
}
```

`flush_interval -1` tells Caddy to flush immediately, never buffer. The default is generally good for SSE, but `-1` is explicit and safe.

In practice you don't need a separate Caddy in front of the Mercure hub — the Mercure binary *is* a Caddy build. You can mount your existing site and the hub on the same Caddy instance with one config:

```caddyfile
# Caddy
example.com {
  route /api/* {
    reverse_proxy api:8080
  }
  route /.well-known/mercure* {
    mercure {
      publisher_jwt {env.MERCURE_PUBLISHER_JWT_KEY}
      subscriber_jwt {env.MERCURE_SUBSCRIBER_JWT_KEY}
    }
  }
  reverse_proxy frontend:3000
}
```

This sidesteps CORS entirely (everything's same-origin) and is the recommended pattern when you don't already have an existing reverse proxy.

## HAProxy

```text
# HAProxy
frontend https
    bind *:443 ssl crt /etc/ssl/hub.example.com.pem alpn h2,http/1.1
    http-request set-header X-Forwarded-Proto https
    default_backend mercure

backend mercure
    option http-server-close
    timeout server 24h           # long-lived SSE
    timeout tunnel 24h
    server mercure 127.0.0.1:8080
```

The `tunnel` timeout is HAProxy's term for how long it lets an idle connection stay open after WebSocket-style upgrade or long-poll. SSE connections look the same way to it.

## AWS ALB

ALBs work with SSE if you bump the idle timeout — default is 60 seconds, which is too short:

- **Idle timeout:** raise to several minutes (e.g. `300`).
- **Connection re-use:** ALBs already do HTTP/2 to clients and HTTP/1.1 to targets, which is fine for SSE.
- **Health checks:** point them at `/.well-known/mercure` (returns 405 — pass any 4xx/5xx as healthy). Or expose the admin health endpoints on a separate listener.

## Cloudflare

Cloudflare proxies SSE, but be aware:

- The free tier has a **100-second proxy timeout** for streaming responses on Free, Pro, Business plans. Heartbeats below that interval keep connections alive.
- Cloudflare Workers cannot proxy SSE for arbitrary durations either; use a regular hostname proxy if you can.
- Disable the Rocket Loader optimization for the hub hostname; it can interfere with `EventSource`.

For long-lived SSE without a 100s cap, [Mercure Cloud](https://mercure.rocks/pricing) terminates connections directly without a proxy in between.

## CORS via reverse proxy

If your hub is on a different origin from your app, you can either configure CORS on the hub (`cors_origins`) or rewrite the request through the proxy so the hub appears same-origin:

```caddyfile
# CORS via reverse proxy
app.example.com {
  route /.well-known/mercure* {
    reverse_proxy hub.internal:80
  }
  route /* {
    reverse_proxy frontend:3000
  }
}
```

Same-origin sidesteps CORS entirely. Same-origin also means the cookie can be `Domain=app.example.com` without any subdomain juggling.

## Common SSE Reverse-Proxy Gotchas with Mercure

| Symptom | Likely cause |
| --- | --- |
| Events arrive in batches every few seconds | Proxy is buffering. Disable it. |
| Connections drop every 30 or 60 seconds | Proxy idle timeout. Raise it. |
| 502 Bad Gateway after a while | Proxy thinks the upstream is dead because no bytes flowed. Lower `heartbeat` on the hub or raise the proxy's read timeout. |
| `EventSource` never connects from the browser | CORS misconfiguration. Check `cors_origins` and the response headers. |

## Set `USE_FORWARDED_HEADERS` Carefully on the Mercure Hub

The hub can read `X-Forwarded-*` and the RFC 7239 `Forwarded` header to know the original client IP and scheme:

```caddyfile
# Set USEFORWARDEDHEADERS Carefully on the Mercure Hub
{
  servers {
    trusted_proxies static 10.0.0.0/8 172.16.0.0/12
  }
}
```

Only trust these headers when the proxy in front of the hub strips or replaces them on every request. If clients can send their own `X-Forwarded-For` and the hub trusts it, your IP-based logic is wrong.

## Next Steps for Mercure Reverse Proxies

- [Configuration](configuration.md) — `cors_origins` and friends.
- [Docker](docker.md) — running the hub in a container.
- [Health monitoring](../production/health-monitoring.md) — what your proxy's health check should hit.
