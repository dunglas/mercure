# Using Multiple Nodes

The free version of the Mercure.rocks Hub is shipped with transports (BoltDB and local) that can only run on a single node.
However, the Mercure.rocks Hub has been designed from the ground up to support clusters.

Both [the Cloud (starting from the Pro plan) and the On Premise versions of the Mercure.rocks Hub](https://mercure.rocks/pricing) natively run on multiple nodes.
These versions are designed for fault tolerance and can support very high loads.

Both versions work by providing extra transports supporting synchronization of several nodes.
They support all features of the free Hub.

If you don't want to purchase a Cloud or an on-premises version of the Mercure.rocks Hub, you can also create your custom build of Mercure.rocks [using a custom transport](https://github.com/dunglas/mercure/blob/main/transport.go#L40-L52).

## Managed Version

[The Cloud version](cloud.md) is hosted on our own High Availability infrastructure (built on top of Kubernetes). This service is 100% hosted and managed: you have nothing to do!

The managed version of the Mercure.rocks Hub can be purchased [directly online](https://mercure.rocks/pricing).
After the purchase, a production-ready Hub is instantly deployed.

To use it, just configure your custom domain name (if any) and your secret JWT key from the administration panel; that's all!

## High Availability On-Prem Version

The High Availability On-Prem Mercure.rocks Hub is a drop-in replacement for the free Hub, which allows you to spread the load across as many servers as you want. It is designed to run on your own servers and is fault-tolerant by default.

The HA version is shipped with transports having node synchronization capabilities.
These transports can rely on:

- Redis or Valkey
- Postgres `LISTEN`/`NOTIFY`
- Apache Kafka
- Apache Pulsar

The on-premises version also has support for enterprise features, including:

- advanced rate limiting
- drop-in replacements builds for [FrankenPHP](https://frankenphp.dev) including High Availability transports

We can help you to decide which synchronization mechanism will be the best suited for your needs, and help you to install and configure it on your infrastructure.

The HA version is provided as binaries and as a Docker image. We also maintain a Helm chart that allows for
installation on any Kubernetes cluster.

For more details (and a benchmark), [read the case studies section](../spec/use-cases.md#case-studies).

### Purchasing

To purchase the On-Prme version of the Mercure.rocks Hub, drop us a mail: [contact@mercure.rocks](mailto:contact@mercure.rocks?subject=I%27m%20interested%20in%20Mercure%20on%20premise)

### Setting the License

A license key is provided when you purchase the High Availability version of the Mercure.rocks Hub.
This key must be set in an environment variable named `MERCURE_LICENSE`.

Ex:

```console
MERCURE_LICENSE=snip \
    MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    ./mercure run
```

If you use the Helm chart, set the `license` value and change the Docker image to use the one provided.

### Transports

The clustered mode of the Mercure.rocks Hub requires transport to work.
Supported transports are Apache Pulsar, Apache Kafka, and PostgreSQL.

#### Redis/Valkey Transport

This is the recommended transport when the Hub isn't the main storage system and low latency is needed.
The Redis transport should fit most use cases.

To install Redis, [read the documentation](https://redis.io/topics/quickstart).
[Valkey](https://valkey.io) is also supported.
Most Cloud Computing platforms also provide managed versions of Redis or Valkey.

| Feature         | Supported |
| --------------- | --------- |
| History         | ✅        |
| Presence API    | ✅        |
| Custom event ID | ✅        |

##### Redis Configuration

The following options can be passed to the `transport` directive:

| Option                   | Description                                                                                                                                             |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `address` or `addresses` | the address(es) of the Redis server(s), you can pass several addresses to use several Redis servers (e.g., `addresses host1:6379 host2:6379`, required) |
| `stream`                 | the name of the Redis stream to use (required)                                                                                                          |
| `password`               | the Redis password                                                                                                                                      |
| `tls`                    | enable TLS support                                                                                                                                      |
| `max_length`             | the approximate maximum number of messages to store in the history, set to `0` to store all messages                                                    |
| `gob`                    | use the Go `gob` encoding instead of JSON (faster but not compatible with third-party systems querying the Redis instance directly)                     |

All [the configuration parameters and formats](https://mercure.rocks/docs/hub/config) supported by the free Mercure.rocks Hub are also available.

###### Reusing an Existing Caddy Storage Redis Instance

[The Redis storage module for Caddy](https://github.com/pberkel/caddy-storage-redis) is shipped with the HA build of Mercure.rocks.

If you use it (for instance, as the storage for [the Caddy ratelimit module](https://github.com/mholt/caddy-ratelimit) that is also included),
you can reuse the same Redis instance for Mercure.rocks by using the special `caddy-storage-redis.alt` value as `address` in your `Caddyfile`:

Here is an example using the built-in environment variables:

<!-- markdownlint-disable MD010 -->

```env
MERCURE_EXTRA_DIRECTIVES="transport redis {
	address caddy-storage-redis.alt
	stream mercure
    # ...
}"
GLOBAL_OPTIONS="storage redis"
```

<!-- markdownlint-enable MD010 -->

###### Legacy Redis URL

**This feature is deprecated: use the new `transport` directive instead**.

The following options can be passed as query parameters of the URL set in `transport_url`:

| Parameter        | Description                                                                                          | Default |
| ---------------- | ---------------------------------------------------------------------------------------------------- | ------- |
| `tls`            | set to `1` to enable TLS support                                                                     | `0`     |
| `max_len_approx` | the approximate maximum number of messages to store in the history, set to `0` to store all messages | `0`     |

#### PostgreSQL Transport

The PostgreSQL Transport allows to store permanently the event and to query them using the full power of SQL.
It is mostly useful when using the Mercure.rocks Hub as an event store, or as a primary data store.

This feature uses PostgreSQL `LISTEN`/`NOTIFY`.

To install PostgreSQL, [read the documentation](https://www.postgresql.org/docs/12/tutorial-install.html).
Most Cloud Computing platforms also provide managed versions of PostgreSQL.

| Feature         | Supported    |
| --------------- | ------------ |
| History         | ✅           |
| Presence API    | ❌ (planned) |
| Custom event ID | ✅           |

##### PostgreSQL Configuration

The following options can be passed to the `transport` directive:

| Option | Description                                                                                                   |
| ------ | ------------------------------------------------------------------------------------------------------------- |
| `url`  | The URL (DSN) to use to connect to Postgres (e.g., `postgres://user:password@127.0.0.1/mercure-ha`, required) |

[Options supported by `libpq`](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING) can be passed as query parameters of the URL set in `_url`.

All [the configuration parameters and formats](https://mercure.rocks/docs/hub/config) supported by the free Mercure.rocks Hub are also available.

###### Legacy PostgreSQL URL

**This feature is deprecated: use the new `transport` directive instead**.

[Options supported by `libpq`](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING) can be passed as query parameters of the URL set in `transport_url`.

#### Kafka Transport

The Kafka transport should only be used when Kafka is already part of your stack.

To install Apache Kafka, [read the quickstart guide](https://kafka.apache.org/quickstart).

Most Cloud Computing platforms also provide managed versions of Kafka.
The Mercure.rocks hub has been tested with:

- Bitnami's Kafka Docker images (Kubernetes and the like)
- Amazon Managed Streaming for Apache Kafka (Amazon MSK)
- IBM Event Streams for IBM Cloud
- Heroku Kafka

| Feature         | Supported |
| --------------- | --------- |
| History         | ✅        |
| Presence API    | ❌        |
| Custom event ID | ✅        |

##### Kafka Configuration

The following options can be passed to the `transport` directive:

| Option                   | Description                                                                                                                                             |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `address` or `addresses` | the address(es) of the Kafka server(s), you can pass several addresses to use several Kafka servers (e.g., `addresses host1:9092 host2:9092`, required) |
| `topic`                  | the name of the Kafka topic to use, **all Mercure.rocks hub instances must use the same topic** (required)                                              |
| `consumer_group`         | the consumer group of this node, **must be different for every instance of the Mercure.rocks hub**                                                      |
| `user`                   | the Kafka SASL user (optional)                                                                                                                          |
| `password`               | the Kafka SASL password (optional)                                                                                                                      |
| `tls`                    | enable TLS support                                                                                                                                      |

All [the configuration parameters and formats](https://mercure.rocks/docs/hub/config) supported by the free Mercure.rocks Hub are also available.

###### Legacy Kafka URL

**This feature is deprecated: use the new `transport` directive instead**.

The following options can be passed as query parameters of the URL set in `transport_url`:

| Parameter        | Description                                                                                                                                   |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `addr`           | addresses of the Kafka servers, you can pass several `addr` parameters to use several Kafka servers (e.g., `addr=host1:9092&addr=host2:9092`) |
| `topic`          | the name of the Kafka topic to use (e.g., `topic=mercure-ha`), **all Mercure.rocks hub instances must use the same topic**                    |
| `consumer_group` | the consumer group of this node, **must be different for every instance of the Mercure.rocks hub** (e.g., `consumer_group=<random-string>`)   |
| `user`           | Kafka SASL user (optional, e.g., `user=kevin`)                                                                                                |
| `password`       | Kafka SASL password (optional, e.g., `password=maman`)                                                                                        |
| `tls`            | Set to `1` to enable TLS (e.g., `tls=1`)                                                                                                      |

#### Pulsar Transport

The Pulsar transport should only be used when Pulsar is already part of your stack.

To install Apache Pulsar, [read the documentation](https://pulsar.apache.org/docs/en/standalone/).

| Feature         | Supported    |
| --------------- | ------------ |
| History         | ✅           |
| Presence API    | ❌           |
| Custom event ID | ❌ (planned) |

##### Pulsar Configuration

The following options can be passed to the `transport` directive:

| Option              | Description                                                                                                 |
| ------------------- | ----------------------------------------------------------------------------------------------------------- |
| `url`               | the address of the Pulsar server (required)                                                                 |
| `topic`             | the name of the Pulsar topic to use, **all Mercure.rocks hub instances must use the same topic** (required) |
| `subscription_name` | the subscription name for this node, **must be different for every instance of the Mercure.rocks hub**      |

All [the configuration parameters and formats](https://mercure.rocks/docs/hub/config) supported by the free Mercure.rocks Hub are also available.

###### Legacy Pulsar URL

**This feature is deprecated: use the new `transport` directive instead**.

The following options can be passed as query parameters of the URL set in `transport_url`:

| Parameters          | Description                                                                                                                                        |     |
| ------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- | --- |
| `topic`             | the name of the Pulsar topic to use (e.g., `topic=mercure`), **all Mercure.rocks hub instances must use the same topic**                           |     |
| `subscription_name` | the subscription name for this node, **must be different for every instance of the Mercure.rocks hub** (e.g., `subscription_name=<random-string>`) |     |

### Docker Images and Kubernetes Chart

An official Docker image and [a Kubernetes Chart](install.md#kubernetes) are available.
Contact us if you need help using them.

### Updates

New releases of the High Availability Mercure.rocks Hub are automatically available in the Amazon S3 bucket containing the binary and on the Docker registry.

## Support

For support requests related to the on-premises version of Mercure.rocks, send a mail to [contact@mercure.rocks](mailto:contact@mercure.rocks?subject=On%20Premise%20support%20request).
