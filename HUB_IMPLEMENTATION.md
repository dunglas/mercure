## Hub Implementation

### Managed Version

A managed, high-scalability version of Mercure is available in private beta.
[Drop us a mail](mailto:dunglas+mercure@gmail.com?subject=I%27m%20interested%20in%20Mercure%27s%20private%20beta) for details and pricing.

### Usage

#### Prebuilt Binary

Grab the binary corresponding to your operating system and architecture [from the release page](https://github.com/dunglas/mercure/releases), then run:

    JWT_KEY='!ChangeMe!' ADDR=':3000' DEMO=1 ALLOW_ANONYMOUS=1 CORS_ALLOWED_ORIGINS=* PUBLISH_ALLOWED_ORIGINS='http://localhost:3000' ./mercure

Note: Mac OS users must use the `Darwin` binary.

The server is now available on `http://localhost:3000`, with the demo mode enabled. Because `ALLOW_ANONYMOUS` is set to `1`, anonymous subscribers are allowed.

To run it in production mode, and generate automatically a Let's Encrypt TLS certificate, just run the following command as root:

    JWT_KEY='!ChangeMe!' ACME_HOSTS='example.com' ./mercure

The value of the `ACME_HOSTS` environment variable must be updated to match your domain name(s).
A Let's Encrypt TLS certificate will be automatically generated.
If you omit this variable, the server will be exposed using a not encrypted HTTP connection.

When the server is up and running, the following endpoints are available:

* `POST https://example.com/hub`: to publish updates
* `GET https://example.com/hub`: to subscribe to updates

See [the protocol](spec/mercure.md) for further informations.

To compile the development version and register the demo page, see [CONTRIBUTING.md](CONTRIBUTING.md#hub).

#### Docker Image

A Docker image is available on Docker Hub. The following command is enough to get a working server in demo mode:

    docker run \
        -e JWT_KEY='!ChangeMe!' -e DEMO=1 -e ALLOW_ANONYMOUS=1 -e PUBLISH_ALLOWED_ORIGINS='http://localhost' \
        -p 80:80 \
        dunglas/mercure

The server, in demo mode, is available on `http://localhost`. Anonymous subscribers are allowed.

In production, run:

    docker run \
        -e JWT_KEY='!ChangeMe!' -e ACME_HOSTS='example.com' \
        -p 80:80 -p 443:443 \
        dunglas/mercure

Be sure to update the value of `ACME_HOSTS` to match your domain name(s), a Let's Encrypt TLS certificate will be automatically generated.

#### Kubernetes

To install Mercure in a [Kubernetes](https://kubernetes.io) cluster, use the official [Helm Chart](https://hub.helm.sh/charts/stable/mercure):

    helm install stable/mercure

### Environment Variables

* `ACME_CERT_DIR`: the directory where to store Let's Encrypt certificates
* `ACME_HOSTS`: a comma separated list of hosts for which Let's Encrypt certificates must be issued
* `ADDR`: the address to listen on (example: `127.0.0.1:3000`, default to `:http` or `:https` depending if HTTPS is enabled or not). Note that Let's Encrypt only supports the default port: to use Let's Encrypt, **do not set this variable**.
* `ALLOW_ANONYMOUS`:  set to `1` to allow subscribers with no valid JWT to connect
* `CERT_FILE`: a cert file (to use a custom certificate)
* `KEY_FILE`: a key file (to use a custom certificate)
* `COMPRESS`: set to `0` to disable HTTP compression support (default to enabled)
* `CORS_ALLOWED_ORIGINS`: a comma separated list of allowed CORS origins, can be `*` for all
* `DB_PATH`: the path of the [bbolt](https://github.com/etcd-io/bbolt) database (default to `updates.db` in the current directory)
* `DEBUG`: set to `1` to enable the debug mode (prints recovery stack traces)
* `DEMO`: set to `1` to enable the demo mode (automatically enabled when `DEBUG=1`)
* `HEARTBEAT_INTERVAL`: interval between heartbeats (useful with some proxies, and old browsers, default to `15s`, set to `0s` to disable)
* `HISTORY_SIZE`: size of the history (to retrieve lost messages using the `Last-Event-ID` header), set to `0` to never remove old events (default)
* `HISTORY_CLEANUP_FREQUENCY`: chances to trigger history cleanup when an update occurs, must be a number between `0` (never cleanup) and `1` (cleanup after every publication), default to `0.3`
* `JWT_KEY`: the JWT key to use for both publishers and subscribers
* `LOG_FORMAT`: the log format, can be `JSON`, `FLUENTD` or `TEXT` (default)
* `PUBLISH_ALLOWED_ORIGINS`: a comma separated list of origins allowed to publish (only applicable when using cookie-based auth)
* `PUBLISHER_JWT_KEY`: must contain the secret key to valid publishers' JWT, can be omited if `JWT_KEY` is set
* `READ_TIMEOUT`: maximum duration for reading the entire request, including the body, set to `0s` to disable (default), example: `2m`
* `SUBSCRIBER_JWT_KEY`: must contain the secret key to valid subscribers' JWT, can be omited if `JWT_KEY` is set
* `WRITE_TIMEOUT`: maximum duration before timing out writes of the response, set to `0s` to disable (default), example: `2m`
* `USE_FORWARDED_HEADERS`: set to `1` to use the `X-Forwarded-For`, and `X-Real-IP` for the remote (client) IP address, `X-Forwarded-Proto` or `X-Forwarded-Scheme` for the scheme (http or https), `X-Forwarded-Host` for the host and the RFC 7239 `Forwarded` header, which may include both client IPs and schemes. If this option is enabled, the reverse proxy must override or remove these headers or you will be at risk.

If `ACME_HOSTS` or both `CERT_FILE` and `KEY_FILE` are provided, an HTTPS server supporting HTTP/2 connection will be started.
If not, an HTTP server will be started (**not secure**).

### Troubleshooting

#### Windows

If you're having trouble getting the Hub running, you may have set an incorrect value for the environment variable `ADDR`. Use `ADDR=":3000"` (and not `ADDR="localhost:3000"`). Windows may ask you for allowing `mercure.exe` in your firewall.

#### 401 Unauthorized

* Check the logs written by the hub on `stderr`, they contain the exact reason why the token has been rejected
* Be sure to set a **secret key** (and not a JWT) in `JWT_KEY` (or in `SUBSCRIBER_JWT_KEY` and `PUBLISHER_JWT_KEY`)
* If the secret key contains special characters, be sure to escape them properly, especially if you set the environment variable in a shell, or in a YAML file (Kubernetes...)
* The publisher always needs a valid JWT, even if `ALLOW_ANONYMOUS` is set to `1`, this JWT **must** have a property named `publish` and containing an array of targets ([example](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOltdfX0.473isprbLWLjXmAaVZj6FIVkCdjn37SQpGjzWws-xa0))
* The subscriber needs a valid JWT only if `ALLOW_ANONYMOUS` is set to `0` (default), or to subscribe to private updates, in this case the JWT **must** have a property named `subscribe` and containing an array of targets ([example](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6W119fQ.s-6MlTvJ6vpsZ7ftmz3dvWpZznRxnxI0KlrZOHVo8Qc))

For both the `publish` and `subscribe` properties, the array can be empty to publish only public updates, or set it to `["*"]` to allow accessing to all targets.

#### Browser Issues

If subscribing to the `EventSource` in the browser doesn't work (the browser instantly disconnects from the stream or complains about CORS policy on receiving an event), check that you've set a proper value for `CORS_ALLOWED_ORIGINS` on running Mercure. It's fine to use `CORS_ALLOWED_ORIGINS=*` for your local development.

#### URI Template and Topics

Try [our URI template tester](https://uri-template-tester.mercure.rocks/) to ensure that the template matches the topic.