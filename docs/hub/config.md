# Configuration

The Mercure Hub is configurable using [environment variables](https://en.wikipedia.org/wiki/Environment_variable) (recommended in production, [twelve-factor app methodology](https://12factor.net/)), command line flags and configuration files (JSON, TOML, YAML, HCL, envfile and Java properties files are supported).

Environment variables must be the name of the configuration parameter in uppercase.
Run `./mercure -h` to see all available command line flags.

Configuration files must be named `mercure.<format>` (ex: `mercure.yaml`) and stored in one of the following directories:

* the current directory (`$PWD`)
* `~/.config/mercure/` (or any other XDG configuration directory set with the `XDG_CONFIG_HOME` environment variable)
* `/etc/mercure`

Most configuration parameters are hot reloaded: changes made to environment variables or configuration files are immediately taken into account, without having to restart the hub.

When using environment variables, list must be space separated. As flags parameters, they must be comma separated.

| Parameter                        | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
|----------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `acme_cert_dir`                  | the directory where to store Let's Encrypt certificates                                                                                                                                                                                                                                                                                                                                                                                                          |
| `acme_hosts`                     | a list of hosts for which Let's Encrypt certificates must be issued                                                                                                                                                                                                                                                                                                                                                                                              |
| `acme_http01_addr`               | the address used by the acme server to listen on (example: `0.0.0.0:8080`), defaults to `:http`.                                                                                                                                                                                                                                                                                                                                                                 |
| `addr`                           | the address to listen on (example: `127.0.0.1:3000`, defaults to `:http` or `:https` depending if HTTPS is enabled or not). Note that Let's Encrypt only supports the default port: to use Let's Encrypt, **do not set this parameter**.                                                                                                                                                                                                                         |
| `allow_anonymous`                | set to `true` to allow subscribers with no valid JWT to connect                                                                                                                                                                                                                                                                                                                                                                                                  |
| `cert_file`                      | a cert file (to use a custom certificate)                                                                                                                                                                                                                                                                                                                                                                                                                        |
| `key_file`                       | a key file (to use a custom certificate)                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `compress`                       | set to `false` to disable HTTP compression support, defaults to enabled                                                                                                                                                                                                                                                                                                                                                                                          |
| `cors_allowed_origins`           | a list of allowed CORS origins, can be `*` for all                                                                                                                                                                                                                                                                                                                                                                                                               |
| `debug`                          | set to `true` to enable the debug mode, **dangerous, don't enable in production** (logs updates' content, why an update is not send to a specific subscriber and recovery stack traces)                                                                                                                                                                                                                                                                          |
| `demo`                           | set to `true` to enable the demo mode (automatically enabled when `debug=true`)                                                                                                                                                                                                                                                                                                                                                                                  |
| `heartbeat_interval`             | interval between heartbeats (useful with some proxies, and old browsers), defaults to `15s`, set to `0s` to disable                                                                                                                                                                                                                                                                                                                                              |
| `transport_url`                  | URL representation of the history database. Provided database are `null` to disabled history, `bolt` to use [bbolt](https://github.com/etcd-io/bbolt) (example `bolt:///var/run/mercure.db?size=100&cleanup_frequency=0.4`), defaults to `bolt://updates.db`                                                                                                                                                                                                     |
| `jwt_key`                        | the JWT key to use for both publishers and subscribers                                                                                                                                                                                                                                                                                                                                                                                                           |
| `jwt_algorithm`                  | the JWT verification algorithm to use for both publishers and subscribers, e.g. HS256 (default) or RS512                                                                                                                                                                                                                                                                                                                                                         |
| `log_format`                     | the log format, can be `JSON`, `FLUENTD` or `TEXT` (default)                                                                                                                                                                                                                                                                                                                                                                                                     |
| `publish_allowed_origins`        | a list of origins allowed to publish (only applicable when using cookie-based auth)                                                                                                                                                                                                                                                                                                                                                                              |
| `publisher_jwt_key`              | must contain the secret key to valid publishers' JWT, can be omitted if `jwt_key` is set                                                                                                                                                                                                                                                                                                                                                                         |
| `publisher_jwt_algorithm`        | the JWT verification algorithm to use for publishers, e.g. HS256 (default) or RS512                                                                                                                                                                                                                                                                                                                                                                              |
| `read_timeout`                   | maximum duration for reading the entire request, including the body, set to `0s` to disable (default), example: `2m`                                                                                                                                                                                                                                                                                                                                             |
| `subscriber_jwt_key`             | must contain the secret key to valid subscribers' JWT, can be omitted if `jwt_key` is set                                                                                                                                                                                                                                                                                                                                                                        |
| `subscriber_jwt_algorithm`       | the JWT verification algorithm to use for subscribers, e.g. HS256 (default) or RS512                                                                                                                                                                                                                                                                                                                                                                             |
| `write_timeout`                  | maximum duration before timing out writes of the response, set to `0s` to disable (default), example: `2m`                                                                                                                                                                                                                                                                                                                                                       |
| `use_forwarded_headers`          | set to `true` to use the `X-Forwarded-For`, and `X-Real-IP` for the remote (client) IP address, `X-Forwarded-Proto` or `X-Forwarded-Scheme` for the scheme (http or https), `X-Forwarded-Host` for the host and the RFC 7239 `Forwarded` header, which may include both client IPs and schemes. If this option is enabled, the reverse proxy must override or remove these headers or you will be at risk                                                        |
| `dispatch_subscriptions`         | set to `true` to dispatch updates when a subscription between the Hub and a subscriber is established or closed. The topic follows the template `https://mercure.rocks/subscriptions/{subscriptionID}`. To receive connection updates, subscribers must have `https://mercure.rocks/targets/subscriptions` or an URL matching the template `https://mercure.rocks/targets/subscriptions/{topic}` (`{topic}` is URL-encoded topic of the subscription) as targets |
| `subscriptions_include_ip`       | set to `true` to include the subscriber's IP in the subscription update                                                                                                                                                                                                                                                                                                                                                                                           |
If `acme_hosts` or both `cert_file` and `key_file` are provided, an HTTPS server supporting HTTP/2 connection will be started.
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

Unix

```
JWT_KEY=`cat jwt_key.pub` ./mercure
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
| `cleanup_frequency` | chances to trigger history cleanup when an update occurs, must be a number between `0` (never cleanup) and `1` (cleanup after every publication), default to `0.3`. |
| `size`              | size of the history (to retrieve lost messages using the `Last-Event-ID` header), set to `0` to never remove old events (default)                                                |

Below are common examples of valid DSNs showing a combination of available values:

    # absolute path to `updates.db`
    transport_url="bolt:///var/run/database.db"

    # path to `updates.db` in the current directory
    transport_url="bolt://database.db"

    # custom options
    transport_url="bolt://database.db?bucket_name=demo&size=1000&cleanup_frequency=0.5"
