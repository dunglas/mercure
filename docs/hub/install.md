# Install the Mercure.rocks hub

## Managed and HA Versions

[Managed and High Availability versions of Mercure.rocks](https://mercure.rocks/pricing) are available, give them a try!

## Prebuilt Binary

First, download the archive corresponding to your operating system and architecture [from the release page](https://github.com/dunglas/mercure/releases), extract the archive and open a shell in the resulting directory.

Note: Mac OS users must use the `Darwin` binary.

Then, on Linux and Mac OS X, run:

    ./mercure --jwt-key='!ChangeMe!' --addr=':3000' --debug --allow-anonymous --cors-allowed-origins='*' --publish-allowed-origins='http://localhost:3000'

On Windows, start PowerShell, go into the extracted directory and run:

    $env:JWT_KEY='!ChangeMe!'; $env:ADDR='localhost:3000'; $env:DEMO='1'; $env:ALLOW_ANONYMOUS='1'; $env:CORS_ALLOWED_ORIGINS='*'; $env:PUBLISH_ALLOWED_ORIGINS='http://localhost:3000'; .\mercure.exe

The Windows Defender Firewall will ask you if you want to allow `mercure.exe` to communicate through it.
Allow it for both public and private networks. If you use an antivirus, or another firewall software, be sure to whitelist `mercure.exe`. 

The server is now available on `http://localhost:3000`, with the demo mode enabled. Because the `allow_anonymous` option is enabled, anonymous subscribers are allowed.

To run it in production mode, and generate automatically a Let's Encrypt TLS certificate, run the following command as root:

    JWT_KEY='!ChangeMe!' ACME_HOSTS='example.com' ./mercure

Using Windows in production is not recommended.

The value of the `ACME_HOSTS` environment variable must be updated to match your domain name(s).
A Let's Encrypt TLS certificate will be automatically generated.
If you omit this variable, the server will be exposed using a not encrypted HTTP connection.

When the server is up and running, the following endpoints are available:

* `POST https://example.com/.well-known/mercure`: to publish updates
* `GET https://example.com/.well-known/mercure`: to subscribe to updates

See [the protocol](spec/mercure.md) for more details about these endpoints.

To compile the development version and register the demo page, see [https://github.com/dunglas/mercure/blob/master/CONTRIBUTING.md](CONTRIBUTING.md#hub).

## Docker Image

### Docker

A Docker image is available on Docker Hub. The following command is enough to get a working server in demo mode:

    docker run \
        -e JWT_KEY='!ChangeMe!' -e DEMO=1 -e ALLOW_ANONYMOUS=1 -e CORS_ALLOWED_ORIGINS=* -e PUBLISH_ALLOWED_ORIGINS='http://localhost' \
        -p 80:80 \
        dunglas/mercure

The server, in demo mode, is available on `http://localhost`. Anonymous subscribers are allowed.

In production, run:

    docker run \
        -e JWT_KEY='!ChangeMe!' -e ACME_HOSTS='example.com' \
        -p 80:80 -p 443:443 \
        dunglas/mercure

Be sure to update the value of `ACME_HOSTS` to match your domain name(s), a Let's Encrypt TLS certificate will be automatically generated.

### Docker Compose

You can use this Docker image in your Compose stack:

```yaml
mercure:
    image: dunglas/mercure
    ports:
        - 80:80
    environment:
        - JWT_KEY=!ChangeMe!
        - DEMO=1
        - ALLOW_ANONYMOUS=1
        - CORS_ALLOWED_ORIGINS=*
        - PUBLISH_ALLOWED_ORIGINS=http://localhost
```

In production:

```yaml
mercure:
    image: dunglas/mercure
    ports:
        - 80:80
        - 443:443
    environment:
        - JWT_KEY=!ChangeMe!
        - ACMS_HOSTS=example.com
```

## Kubernetes

To install Mercure.rocks in a [Kubernetes](https://kubernetes.io) cluster, use the official [Helm Chart](https://hub.helm.sh/charts/stable/mercure):

    helm install stable/mercure

## Arch Linux

Mercure.rocks is available [on the AUR](https://aur.archlinux.org/packages/mercure), you can install it with your favorite AUR wrapper:

    yay -S mercure

Or download the `PKGBUILD` and compile and install it: `makepkg -sri`.
