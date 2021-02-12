# Install the Mercure.rocks hub

## Managed and HA Versions

[Managed and High Availability versions of Mercure.rocks](https://mercure.rocks/pricing) are available, give them a try!

## Prebuilt Binary

The Mercure.rocks hub is available as a custom build of the [Caddy web server](https://caddyserver.com/) including the Mercure.rocks module.

First, download the archive corresponding to your operating system and architecture [from the release page](https://github.com/dunglas/mercure/releases), extract the archive and open a shell in the resulting directory.

*Note:* Mac OS users must use the `Darwin` binary.

Then, to start the Mercure.rocks Hub in development mode on Linux and Mac OS X, run:

    MERCURE_PUBLISHER_JWT_KEY='!ChangeMe!' \
    MERCURE_SUBSCRIBER_JWT_KEY='!ChangeMe!' \
    ./mercure run -config Caddyfile.dev

On Windows, start PowerShell, go into the extracted directory and run:

    $env:MERCURE_PUBLISHER_JWT_KEY='!ChangeMe!'; $env:MERCURE_SUBSCRIBER_JWT_KEY='!ChangeMe!'; .\mercure.exe run -config Caddyfile.dev

*Note:* The Windows Defender Firewall will ask you if you want to allow `mercure.exe` to communicate through it.
Allow it for both public and private networks. If you use an antivirus, or another firewall software, be sure to whitelist `mercure.exe`.

The server is now available on `https://localhost` (TLS is automatically enabled, [learn how to disable it](config.md)).
In development mode, anonymous subscribers are allowed and the debug UI is available on `https://localhost/.well-known/mercure/ui/`.

*Note:* if you get an error similar to `bind: address already in use`, it means that the port `80` or `443` is already used by another service (the usual suspects are Apache and NGINX). Before starting Mercure, stop the service using the port(s) first, or set the `SERVER_NAME` environment variable to use a free port (ex: `SERVER_NAME=:3000`).

To run the server in production mode, run this command:

    MERCURE_PUBLISHER_JWT_KEY='!ChangeMe!' \
    MERCURE_SUBSCRIBER_JWT_KEY='!ChangeMe!' \
    ./mercure run

In production mode, the debugger UI is disabled and anonymous subscribers aren't allowed.
To change these default settings, [learn how to configure the Mercure.rocks hub](config.md).

When the server is up and running, the following endpoints are available:

* `POST https://localhost/.well-known/mercure`: to publish updates
* `GET https://localhost/.well-known/mercure`: to subscribe to updates

See [the protocol](../../spec/mercure.md) for more details about these endpoints.

To compile the development version, see [https://github.com/dunglas/mercure/blob/master/CONTRIBUTING.md](https://github.com/dunglas/mercure/blob/main/CONTRIBUTING.md).

## Docker Image

A Docker image is available on Docker Hub. The following command is enough to get a working server in development mode:

    docker run \
        -e MERCURE_PUBLISHER_JWT_KEY='!ChangeMe!' \
        -e MERCURE_SUBSCRIBER_JWT_KEY='!ChangeMe!' \
        -p 80:80 \
        -p 443:443 \
        dunglas/mercure caddy run -config /etc/caddy/Caddyfile.dev

The server is then available on `https://localhost`. Anonymous subscribers are allowed and the debugger UI is available on `https://localhost/.well-known/mercure/ui/`.

In production, simply run:

    docker run \
        -e MERCURE_PUBLISHER_JWT_KEY='!ChangeMe!' \
        -e MERCURE_SUBSCRIBER_JWT_KEY='!ChangeMe!' \
        -p 80:80 \
        -p 443:443 \
        dunglas/mercure

HTTPS support is automatically enabled. If you run the Mercure hub behind a reverse proxy [such as NGINX](cookbooks.md#using-nginx-as-an-http-2-reverse-proxy-in-front-of-the-hub), you usually want to use unencrypted HTTP.
This can be done like that:

    docker run \
        -e SERVER_NAME=':80' \
        -e MERCURE_PUBLISHER_JWT_KEY='!ChangeMe!' \
        -e MERCURE_SUBSCRIBER_JWT_KEY='!ChangeMe!' \
        -p 80:80 \
        dunglas/mercure

The Docker image is based on [the Caddy Server Docker image](https://registry.hub.docker.com/_/caddy).
See [the configuration section](config.md) and [the documentation of the Docker image for Caddy](https://registry.hub.docker.com/_/caddy) to learn how to configure it to fit your needs.

## Docker Compose

If you prefer to use `docker-compose` to run the Mercure.rocks hub, here's a sample service definition:

```yaml
# docker-compose.yml
version: "3.7"

services:
  caddy:
    image: dunglas/mercure
    restart: unless-stopped
    environment:
      # Uncomment the following line to disable HTTPS
      #SERVER_NAME: ':80'
      MERCURE_PUBLISHER_JWT_KEY: '!ChangeMe!'
      MERCURE_SUBSCRIBER_JWT_KEY: '!ChangeMe!'
    # Uncomment the following line to enable the development mode
    #command: /usr/bin/caddy run -config /etc/caddy/Caddyfile.dev
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - caddy_data:/data
      - caddy_config:/config

volumes:
  caddy_data:
  caddy_config:
```

## Arch Linux

Mercure.rocks is available [on the AUR](https://aur.archlinux.org/packages/mercure), you can install it with your favorite AUR wrapper:

    yay -S mercure

Or download the `PKGBUILD` and compile and install it: `makepkg -sri`.

## Custom Caddy Build

It's also possible to [download Caddy with Mercure and other modules included](https://caddyserver.com/download?package=github.com%2Fdunglas%2Fmercure%2Fcaddy), or to build your own binaries using [`xcaddy`](https://github.com/caddyserver/xcaddy):

    xcaddy build --with github.com/dunglas/mercure/caddy

## Integrations in Popular Frameworks

The Mercure.rocks is shipped by [several popular services and frameworks](../ecosystem/awesome.md#frameworks-and-services-integrations), including Symfony and API Platform.
Refer to their documentations to get started.
