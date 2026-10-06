---
title: "Run Mercure behind Caddy, NGINX, Traefik, HAProxy, or Cloudflare"
description: "Reverse-proxy the Mercure.rocks Hub behind Caddy, NGINX, Traefik, HAProxy, AWS ALB, or Cloudflare with SSE-friendly buffering and timeouts."
---

# Mercure reverse proxies

Mercure works behind a reverse proxy that supports streaming HTTP responses. Disable response buffering and keep the idle timeout longer than the hub's heartbeat interval (40 seconds by default).

1. **Don't buffer the response.** SSE pushes events as they're written; buffering holds them until the buffer fills, which delays everything by seconds.
2. **Long read timeouts.** A typical 30s or 60s read timeout closes every SSE connection that goes idle.

The examples below need your hostnames, certificates, and issuer configuration. For a TLS-terminating proxy, also configure the hub's [public resource identifier](#configure-trusted-proxies-and-the-public-hub-url).

## Caddy

If you're terminating TLS in another Caddy instance (or fronting Mercure with a separate Caddy reverse proxy):

```caddyfile
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

Caddy flushes SSE responses immediately based on their `text/event-stream` content type. `flush_interval -1` also requests immediate flushing for other responses.

In practice you don't need a separate Caddy in front of the Mercure hub, the Mercure binary _is_ a Caddy build. You can mount your existing site and the hub on the same Caddy instance with one config:

```caddyfile
example.com {
  route /api/* {
    reverse_proxy api:8080
  }
  route /.well-known/mercure* {
    mercure {
      issuer https://example.com {
        publisher {
          jwt {env.MERCURE_PUBLISHER_JWT_KEY}
        }
        subscriber {
          jwt {env.MERCURE_SUBSCRIBER_JWT_KEY}
        }
      }
    }
  }
  reverse_proxy frontend:3000
}
```

This sidesteps CORS entirely (everything's same-origin) and is the recommended pattern when you don't already have an existing reverse proxy.

## NGINX

```nginx
server {
    listen 443 ssl;
    http2 on;
    server_name hub.example.com;

    ssl_certificate     /etc/ssl/hub.example.com.crt;
    ssl_certificate_key /etc/ssl/hub.example.com.key;

    location / {
        proxy_pass http://mercure-upstream;
        proxy_http_version 1.1;

        # Don't buffer the response, SSE relies on immediate flush
        proxy_buffering off;
        proxy_cache off;

        # Long read timeout, SSE connections live for hours
        proxy_read_timeout 24h;

        # Preserve the public host for routing
        proxy_set_header Connection "";
        proxy_set_header Host $host;
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

- `proxy_buffering off`: without this, NGINX may hold events for seconds until the buffer fills. The single most common cause of "events arrive in batches."
- `proxy_read_timeout 24h`: NGINX's default is 60 seconds. Anything less than your `heartbeat` setting (40s default on the hub) will drop connections regularly.
- `proxy_http_version 1.1` and `Connection ""`: configure the NGINX-to-hub connection. Client-to-NGINX HTTP/2 is configured separately.

## Traefik

This Compose example routes HTTPS traffic through Traefik to the hub:

```yaml
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
      SERVER_NAME: ":80" # let Traefik handle TLS
      MERCURE_PUBLISHER_JWT_KEY: ${MERCURE_PUBLISHER_JWT_KEY}
      MERCURE_SUBSCRIBER_JWT_KEY: ${MERCURE_SUBSCRIBER_JWT_KEY}
      MERCURE_TRUSTED_ISSUERS: https://app.example.com
      MERCURE_EXTRA_DIRECTIVES: |
        resource_identifier https://hub.example.com/.well-known/mercure
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

Disable the hub's own TLS (`SERVER_NAME=:80`). Traefik handles it.

The following setting controls response flushing, not write timeouts:

```yaml
labels:
  - "traefik.http.services.mercure.loadbalancer.responseforwarding.flushinterval=1ms"
```

Traefik flushes recognized streaming responses immediately. Its `flushinterval` setting applies to other responses; see [Traefik response forwarding](https://doc.traefik.io/traefik/reference/routing-configuration/http/load-balancing/service/).

## HAProxy

```text
frontend https
    mode http
    timeout client 24h
    bind *:443 ssl crt /etc/ssl/hub.example.com.pem alpn h2,http/1.1
    http-request set-header X-Forwarded-Proto https
    default_backend mercure
    mode http
    timeout connect 5s

backend mercure
    mode http
    timeout connect 5s
    option http-server-close
    timeout server 24h           # long-lived SSE
    server mercure 127.0.0.1:8080
```

For SSE, configure HTTP client and server timeouts. `timeout tunnel` applies to upgraded connections and CONNECT tunnels, not ordinary SSE responses.

## AWS ALB

ALBs support SSE. Their default 60-second idle timeout exceeds the hub's default 40-second heartbeat, but allow margin for delays:

- **Idle timeout:** raise to several minutes (e.g. `300`).
- **Connection re-use:** ALBs already do HTTP/2 to clients and HTTP/1.1 to targets, which is fine for SSE.
- **Health checks:** expose a restricted route that forwards only the readiness check and require `200`. Do not treat arbitrary `4xx` or `5xx` responses as healthy, or expose the full Caddy admin API.

## Cloudflare

Cloudflare can proxy SSE. Configure the hub's heartbeat below the applicable read timeout, bypass caching for the hub endpoint, and test idle streams through your actual deployment.

Limits depend on the product and plan. Consult [Cloudflare connection limits](https://developers.cloudflare.com/fundamentals/reference/connection-limits/) and, if using Workers, [Workers limits](https://developers.cloudflare.com/workers/platform/limits/) rather than treating a proxy read timeout as a maximum stream lifetime.

## CORS via reverse proxy

If your hub is on a different origin from your app, you can either configure CORS on the hub (`cors_origins`) or rewrite the request through the proxy so the hub appears same-origin:

```caddyfile
app.example.com {
  route /.well-known/mercure* {
    reverse_proxy hub.internal:80
  }
  route /* {
    reverse_proxy frontend:3000
  }
}
```

Serving the hub on the application's origin avoids CORS. A host-only authorization cookie is sufficient; no `Domain` attribute is needed.

## Common SSE reverse-proxy gotchas with Mercure

| Symptom                                       | Likely cause                                                                                                               |
| --------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| Events arrive in batches every few seconds    | Proxy is buffering. Disable it.                                                                                            |
| Connections drop every 30 or 60 seconds       | Proxy idle timeout. Raise it.                                                                                              |
| 502 Bad Gateway after a while                 | Proxy thinks the upstream is dead because no bytes flowed. Lower `heartbeat` on the hub or raise the proxy's read timeout. |
| `EventSource` never connects from the browser | CORS misconfiguration. Check `cors_origins` and the response headers.                                                      |

## Configure trusted proxies and the public hub URL

Caddy can derive client IP addresses from trusted `X-Forwarded-For` headers. Configure trusted proxy addresses:

```caddyfile
{
  servers {
    trusted_proxies static 10.0.0.0/8 172.16.0.0/12
  }
}
```

Only trust these headers when the proxy in front of the hub strips or replaces them on every request. If clients can send their own `X-Forwarded-For` and the hub trusts it, your IP-based logic is wrong.

`trusted_proxies` resolves the client IP only. The hub derives the scheme of its own public identity from the connection it terminates, so behind a proxy that terminates TLS it derives `http://`: the OAuth 2.0 resource identifier it expects as the token `aud`, and the RFC 9728 metadata URL it advertises, are `http://` too.

Set `resource_identifier` to the public `https://` URL to fix that:

```caddyfile
mercure {
  resource_identifier https://example.com/.well-known/mercure
}
```

`public_urls` does not fix it, and does not belong here: it pins the scheme of the origin the hub _receives_, so listing the `https://` form on a hub that is reached over plain HTTP rejects every request with `421 Misdirected Request`. Use it on a catch-all site block reached directly over TLS.

## Next steps

- [Configuration](configuration.md): CORS and issuer settings.
- [Docker](docker.md): running the hub in a container.
- [Health monitoring](../production/health-monitoring.md): what your proxy's health check should hit.
