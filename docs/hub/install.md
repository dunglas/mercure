# Install the Mercure.rocks hub

## Managed and HA Versions

[Managed and High Availability versions of Mercure.rocks](https://mercure.rocks/pricing) are available, give them a try!

## Prebuilt Binary

The Mercure.rocks hub is available as a custom build of the [Caddy web server](https://caddyserver.com/) including the Mercure.rocks module.

First, download the archive corresponding to your operating system and architecture [from the release page](https://github.com/dunglas/mercure/releases), extract the archive and open a shell in the resulting directory.

Note: Mac OS users must use the `Darwin` binary.

Then, on Linux and Mac OS X, run:

    ./mercure run -config Caddyfile.dev

On Windows, start PowerShell, go into the extracted directory and run:

    .\mercure.exe run -config Caddyfile.dev

The Windows Defender Firewall will ask you if you want to allow `mercure.exe` to communicate through it.
Allow it for both public and private networks. If you use an antivirus, or another firewall software, be sure to whitelist `mercure.exe`. 

The server is now available on `https://localhost`, with the demo mode enabled. Because the `allow_anonymous` directive is set in the provided configuration, anonymous subscribers are allowed.

To run the server in production, see [how to configure the Mercure.rocks hub](config.md).

When the server is up and running, the following endpoints are available:

* `POST https://example.com/.well-known/mercure`: to publish updates
* `GET https://example.com/.well-known/mercure`: to subscribe to updates

In demo mode, an UI is also available: `https://example.com/.well-known/mercure/ui`.

See [the protocol](../../spec/mercure.md) for more details about these endpoints.

To compile the development version, see [https://github.com/dunglas/mercure/blob/master/CONTRIBUTING.md](https://github.com/dunglas/mercure/blob/main/CONTRIBUTING.md).

## Custom Caddy Build

It's also possible to build your own binaries containing other [Caddy modules](https://caddyserver.com/download) using [`xcaddy`](https://github.com/caddyserver/xcaddy).

    xcaddy build --with github.com/dunglas/mercure

## Docker Image

### Docker

A Docker image is available on Docker Hub. The following command is enough to get a working server in demo mode:

    docker run -p 80:80 -p 443:443 dunglas/mercure caddy run -config /etc/caddy/Caddyfile.dev

The server, in demo mode, is available on `https://localhost`. Anonymous subscribers are allowed.

In production, simply run:

    docker run -p 80:80 -p 443:443 dunglas/mercure

The Docker image is based on the Caddy server Docker image.
See [the configuration section](config.md) and [the documentation of the Docker image for Caddy](https://registry.hub.docker.com/_/caddy) to learn how to configure it to fit your needs.

## Arch Linux

Mercure.rocks is available [on the AUR](https://aur.archlinux.org/packages/mercure), you can install it with your favorite AUR wrapper:

    yay -S mercure

Or download the `PKGBUILD` and compile and install it: `makepkg -sri`.
