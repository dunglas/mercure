# Use the Mercure.rocks Hub with Traefik Proxy

[Traefik](https://doc.traefik.io/traefik/) is a free and open source *edge router* poular in the Docker and Kubernetes ecosystems.

The following Docker Compose file exposes a Mercure.rocks hub through Traefik:

```yaml
# compose.yaml
services:
  reverse-proxy:
    # The official v2 Traefik image
    image: traefik:v2.10
    command: --api.insecure=true --providers.docker
    ports:
      # The HTTP port
      - '80:80'
      # The Web UI (enabled by --api.insecure=true)
      - '8080:8080'
    volumes:
      # So that Traefik can listen to the Docker events
      - /var/run/docker.sock:/var/run/docker.sock

  mercure:
    # The official Mercure image
    image: dunglas/mercure
    restart: unless-stopped
    environment:
      # Disables Mercure.rocks auto-HTTPS feature, HTTPS must be handled at edge by Traefik or another proxy in front of it
      SERVER_NAME: ':80'
      MERCURE_PUBLISHER_JWT_KEY: '!ChangeThisMercureHubJWTSecretKey!'
      MERCURE_SUBSCRIBER_JWT_KEY: '!ChangeThisMercureHubJWTSecretKey!'
    # Enables the development mode, comment the following line to run the hub in prod mode
    command: /usr/bin/caddy run --config /etc/caddy/dev.Caddyfile
    healthcheck:
      test: ["CMD", "curl", "-f", "https://localhost/healthz"]
      timeout: 5s
      retries: 5
      start_period: 60s
    volumes:
      - mercure_data:/data
      - mercure_config:/config
    labels:
      - "traefik.http.routers.mercure.rule=Host(`mercure.docker.localhost`)"

volumes:
  mercure_data:
  mercure_config:
```

Refer to the Traefik Proxy documentation to learn about all features provided by Traefik.
