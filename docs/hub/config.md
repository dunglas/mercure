# Configuration

The Mercure Hub follows [the twelve-factor app methodology](https://12factor.net/) and is configurable using [environment variables](https://en.wikipedia.org/wiki/Environment_variable):


| Variable                    | Description                                                                                                                                                                                                                                                                                                                                                                                             |
|-----------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `ACME_CERT_DIR`             | the directory where to store Let's Encrypt certificates                                                                                                                                                                                                                                                                                                                                                 |
| `ACME_HOSTS`                | a comma separated list of hosts for which Let's Encrypt certificates must be issued                                                                                                                                                                                                                                                                                                                     |
| `ACME_HTTP01_ADDR`          | the address used by the acme server to listen on (example:  `0.0.0.0:8080`), default to `:http`.                                                                                                                                                                                                                                                                                                        |
| `ADDR`                      | the address to listen on (example: `127.0.0.1:3000`, default to `:http` or `:https` depending if HTTPS is enabled or not). Note that Let's Encrypt only supports the default port: to use Let's Encrypt, **do not set this variable**.                                                                                                                                                                  |
| `ALLOW_ANONYMOUS`           | set to `1` to allow subscribers with no valid JWT to connect                                                                                                                                                                                                                                                                                                                                            |
| `CERT_FILE`                 | a cert file (to use a custom certificate)                                                                                                                                                                                                                                                                                                                                                               |
| `KEY_FILE`                  | a key file (to use a custom certificate)                                                                                                                                                                                                                                                                                                                                                                |
| `COMPRESS`                  | set to `0` to disable HTTP compression support (default to enabled)                                                                                                                                                                                                                                                                                                                                     |
| `CORS_ALLOWED_ORIGINS`      | a comma separated list of allowed CORS origins, can be `*` for all                                                                                                                                                                                                                                                                                                                                      |
| `DEBUG`                     | set to `1` to enable the debug mode, **dangerous, don't enable in production** (logs updates' content, why an update is not send to a specific subscriber and recovery stack traces)                                                                                                                                                                                                                    |
| `DEMO`                      | set to `1` to enable the demo mode (automatically enabled when `DEBUG=1`)                                                                                                                                                                                                                                                                                                                               |
| `HEARTBEAT_INTERVAL`        | interval between heartbeats (useful with some proxies, and old browsers, default to `15s`, set to `0s` to disable)                                                                                                                                                                                                                                                                                      |
| `TRANSPORT_URL`             | URL representation of the history database. Provided database are `null` to disabled history, `bolt` to use [bbolt](https://github.com/etcd-io/bbolt) (example `bolt:///var/run/mercure.db?size=100&cleanup_frequency=10`). (default to `bolt://updates.db`)                                             |
| `JWT_KEY`                   | the JWT key to use for both publishers and subscribers                                                                                                                                                                                                                                                                                                                                                  |
| `JWT_ALGORITHM`             | the JWT verification algorithm to use for both publishers and subscribers, e.g. HS256 or RS512. Defaults to HS256.                                                                                                                                                                                                                                                                                      |
| `LOG_FORMAT`                | the log format, can be `JSON`, `FLUENTD` or `TEXT` (default)                                                                                                                                                                                                                                                                                                                                            |
| `PUBLISH_ALLOWED_ORIGINS`   | a comma separated list of origins allowed to publish (only applicable when using cookie-based auth)                                                                                                                                                                                                                                                                                                     |
| `PUBLISHER_JWT_KEY`         | must contain the secret key to valid publishers' JWT, can be omited if `JWT_KEY` is set                                                                                                                                                                                                                                                                                                                 |
| `PUBLISHER_JWT_ALGORITHM`   | the JWT verification algorithm to use for publishers, e.g. HS256 or RS512. Defaults to HS256.                                                                                                                                                                                                                                                                                                           |
| `READ_TIMEOUT`              | maximum duration for reading the entire request, including the body, set to `0s` to disable (default), example: `2m`                                                                                                                                                                                                                                                                                    |
| `SUBSCRIBER_JWT_KEY`        | must contain the secret key to valid subscribers' JWT, can be omited if `JWT_KEY` is set                                                                                                                                                                                                                                                                                                                |
| `SUBSCRIBER_JWT_ALGORITHM`  | the JWT verification algorithm to use for subscribers, e.g. HS256 or RS512. Defaults to HS256.                                                                                                                                                                                                                                                                                                          |
| `WRITE_TIMEOUT`             | maximum duration before timing out writes of the response, set to `0s` to disable (default), example: `2m`                                                                                                                                                                                                                                                                                              |
| `USE_FORWARDED_HEADERS`     | set to `1` to use the `X-Forwarded-For`, and `X-Real-IP` for the remote (client) IP address, `X-Forwarded-Proto` or `X-Forwarded-Scheme` for the scheme (http or https), `X-Forwarded-Host` for the host and the RFC 7239 `Forwarded` header, which may include both client IPs and schemes. If this option is enabled, the reverse proxy must override or remove these headers or you will be at risk. |

If `ACME_HOSTS` or both `CERT_FILE` and `KEY_FILE` are provided, an HTTPS server supporting HTTP/2 connection will be started.
If not, an HTTP server will be started (**not secure**).

When using RSA public keys for verification make sure the key is properly formatted.

```
-----BEGIN PUBLIC KEY-----
MIGeMA0GCSqGSIb3DQEBAQUAA4GMADCBiAKBgHVwuJsFmzsFnOkGj+OgAp4lTNqR
CF0RZSmjY+ECWOJ3sSEzQ8qtkJe61uSjr/PKmqvBxxex0YtUL7waSS4jvq3ws8Bm
WIxK2GqoAVjLjK8HzThSPQpgv2AjiEXD6iAERHeySLGjYAUgfMrVJ01J5fNSL+O+
bCd7nPuNAyYHCOOHAgMBAAE=
-----END PUBLIC KEY-----
```

Bash
```
JWT_KEY=`cat jwt_key.pub` ./mecure
```

PowerShell
```
$env:JWT_KEY = [IO.File]::ReadAllText(".\jwt_key.pub")
```

## Bolt Adapter

The [Data Source Name (DSN)](https://en.wikipedia.org/wiki/Data_source_name) specifies the path to the [bolt](https://github.com/etcd-io/bbolt) database as well as options

| Parameter           | Description
|---------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `bucket_name`       | name of the bolt bucket to store events. default to `updates`                                                                                                                    |
| `cleanup_frequency` | chances to trchances to trigger history cleanup when an update occurs, must be a number between `0` (never cleanup) and `1` (cleanup after every publication), default to `0.3`. |
| `size`              | size of the history (to retrieve lost messages using the `Last-Event-ID` header), set to `0` to never remove old events (default)                                                |

Below are common examples of valid DSNs showing a combination of available values:

```bash
# absolute path to `updates.db`
TRANSPORT_URL="bolt:///var/run/database.db"

# path to `updates.db` in the current directory
TRANSPORT_URL="bolt://database.db"

# custom options
TRANSPORT_URL="bolt://database.db?bucket_name=demo&size=1000&cleanup_frequency=0.5"
```
