# Install the Mercure.rocks Hub

## Managed and HA Versions

The easiest way to get started with Mercure is to subscribe to the [Cloud version](https://mercure.rocks/pricing).
Give it a try!

## Prebuilt Binary

The Mercure.rocks hub is available as a custom build of the [Caddy web server](https://caddyserver.com/) including the Mercure.rocks module.

First, download the archive corresponding to your operating system and architecture [from the release page](https://github.com/dunglas/mercure/releases), extract the archive and open a shell in the resulting directory.

*Note:* macOS users must download the `Darwin` binary, then run `xattr -d com.apple.quarantine ./mercure` [to release the hub from quarantine](troubleshooting.md#macos-localhost-installation-error).

To start the Mercure.rocks Hub in development mode on Linux and macOS, run:

```console
MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
./mercure run --config dev.Caddyfile
```

On Windows, start PowerShell, go into the extracted directory and run:

```powershell
$env:MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!'; $env:MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!'; .\mercure.exe run --config dev.Caddyfile
```

*Note:* The Windows Defender Firewall will ask you if you want to allow `mercure.exe` to communicate through it.
Allow it for both public and private networks. If you use an antivirus, or another firewall software, be sure to whitelist `mercure.exe`.

The server is now available on `https://localhost` (TLS is automatically enabled, [learn how to disable it](config.md)).
In development mode, anonymous subscribers are allowed and the debug UI is available on `https://localhost/.well-known/mercure/ui/`.

*Note:* if you get an error similar to `bind: address already in use`, it means that the port `80` or `443` is already used by another service (the usual suspects are Apache and NGINX). Before starting Mercure, stop the service using the port(s) first, or set the `SERVER_NAME` environment variable to use a free port (ex: `SERVER_NAME=:3000`).

To run the server in production mode, run this command:

```console
MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
./mercure run
```

In production mode, the debugger UI is disabled and anonymous subscribers aren't allowed.
To change these default settings, [learn how to configure the Mercure.rocks hub](config.md).

When the server is up and running, the following endpoints are available:

* `POST https://localhost/.well-known/mercure`: to publish updates
* `GET https://localhost/.well-known/mercure`: to subscribe to updates

See [the protocol](../../spec/mercure.md) for more details about these endpoints.

To compile the development version, see [https://github.com/dunglas/mercure/blob/master/CONTRIBUTING.md](https://github.com/dunglas/mercure/blob/main/CONTRIBUTING.md).

## Docker Image

A Docker image is available on Docker Hub. The following command is enough to get a working server in development mode:

```console
docker run \
    -e MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    -e MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    -p 80:80 \
    -p 443:443 \
    dunglas/mercure caddy run --config /etc/caddy/dev.Caddyfile
```

The server is then available on `https://localhost`. Anonymous subscribers are allowed and the debugger UI is available on `https://localhost/.well-known/mercure/ui/`.

In production, simply run:

```console
docker run \
    -e MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    -e MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    -p 80:80 \
    -p 443:443 \
    dunglas/mercure
```

HTTPS support is automatically enabled. If you run the Mercure hub behind a reverse proxy [such as NGINX](cookbooks.md#using-nginx-as-an-http-2-reverse-proxy-in-front-of-the-hub), you usually want to use unencrypted HTTP.
This can be done like that:

```console
docker run \
    -e SERVER_NAME=':80' \
    -e MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    -e MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    -p 80:80 \
    dunglas/mercure
```

The Docker image is based on [the Caddy Server Docker image](https://registry.hub.docker.com/_/caddy).
See [the configuration section](config.md) and [the documentation of the Docker image for Caddy](https://registry.hub.docker.com/_/caddy) to learn how to configure it to fit your needs.

## Kubernetes

Use [the Helm package manager](https://helm.sh/) to install Mercure on a Kubernetes cluster:

To install the chart with the release name `my-release`, run the following commands:

```console
helm repo add mercure https://charts.mercure.rocks
helm install my-release mercure/mercure
```

See [the list of available values](https://github.com/dunglas/mercure/blob/main/charts/mercure/README.md) for this chart.

## Docker Compose

If you prefer to use `docker compose` to run the Mercure.rocks hub, here's a sample service definition:

```yaml
# compose.yaml
services:
  mercure:
    image: dunglas/mercure
    restart: unless-stopped
    environment:
      # Uncomment the following line to disable HTTPS
      #SERVER_NAME: ':80'
      MERCURE_PUBLISHER_JWT_KEY: '!ChangeThisMercureHubJWTSecretKey!'
      MERCURE_SUBSCRIBER_JWT_KEY: '!ChangeThisMercureHubJWTSecretKey!'
    # Uncomment the following line to enable the development mode
    #command: /usr/bin/caddy run --config /etc/caddy/dev.Caddyfile
    healthcheck:
      test: ["CMD", "curl", "-f", "https://localhost/healthz"]
      timeout: 5s
      retries: 5
      start_period: 60s
    ports:
      - '80:80'
      - '443:443'
    volumes:
      - mercure_data:/data
      - mercure_config:/config

volumes:
  mercure_data:
  mercure_config:
```

Alternatively, you may want to [run the Mercure.rocks hub behind Traefik Proxy](traefik.md).

## Arch Linux

Mercure.rocks is available [on the AUR](https://aur.archlinux.org/packages/mercure), you can install it with your favorite AUR wrapper:

```console
yay -S mercure
```
Or download the `PKGBUILD` and compile and install it: `makepkg -sri`.

## Custom Caddy Build

It's also possible to [download Caddy with Mercure and other modules included](https://caddyserver.com/download?package=github.com%2Fdunglas%2Fmercure%2Fcaddy), or to build your own binaries using [`xcaddy`](https://github.com/caddyserver/xcaddy):

```console
xcaddy build \
  --with github.com/dunglas/mercure/caddy
```

## Integrations in Popular Frameworks

The Mercure.rocks is shipped by [several popular services and frameworks](../ecosystem/awesome.md#frameworks-and-services-integrations), including Symfony and API Platform.
Refer to their documentations to get started.
