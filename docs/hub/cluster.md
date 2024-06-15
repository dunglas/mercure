# Using Multiple Nodes

The free version of the Mercure.rocks Hub is shipped with transports (BoltDB and local) that can only run on a single node.
However, the Mercure.rocks Hub has been designed from the ground up to support clusters.

Both [the Cloud (starting from the Pro plan) and the On Premise versions of the Mercure.rocks Hub](https://mercure.rocks/pricing) natively run on multiple nodes.
These versions are designed for fault tolerance and can support very high loads.

Both versions work by providing extra transports supporting synchronization of several nodes.
They support all features of the free Hub.

If you don't want to purchase a Cloud or an On Premise version of the Mercure.rocks Hub, you can also create your custom build of Mercure.rocks [using a custom transport](https://github.com/dunglas/mercure/blob/main/transport.go#L40-L52).

## Managed Version

[The Cloud version](cloud.md) is hosted on our own High Availability infrastructure (built on top of Kubernetes). This service is 100% hosted and managed: you have nothing to do!

The managed version of the Mercure.rocks Hub can be purchased [directly online](https://mercure.rocks/pricing).
After the purchase, a production-ready Hub is instantly deployed.

To use it, just configure your custom domain name (if any) and your secret JWT key from the administration panel, that's all!

## High Availability On Premise Version

The High Availability On Premise Mercure.rocks Hub is a drop-in replacement for the free Hub which allows to spread the load across as many servers as you want. It is designed to run on your own servers and is fault tolerant by default.

The HA version is shipped with transports having node synchronization capabilities.
These transports can rely on:

* Redis
* Postgres `LISTEN`/`NOTIFY`
* Apache Kafka
* Apache Pulsar

We can help you to decide which synchronization mechanism will be the best suited for your needs, and help you to install and configure it on your infrastructure.

The HA version is provided as binaries and as a Docker image. We also maintain a Helm chart allowing to install it
on any Kubernetes cluster.

For more details (and a benchmark), [read the case studies section](../spec/use-cases.md#case-studies).

### Purchasing

To purchase the On Premise version of the Mercure.rocks Hub, drop us a mail: [contact@mercure.rocks](mailto:contact@mercure.rocks?subject=I%27m%20interested%20in%20Mercure%20on%20premise)

### Setting the License

A license key is provided when you purchase the High Availability version of the Mercure.rocks Hub.
This key must be set in an environment variable named `MERCURE_LICENSE`.

Ex:

    MERCURE_LICENSE=snip \
    MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    ./mercure run

If you use the Helm chart, set the `license` value and change the Docker image to use the one provided.

### Transports

The clustered mode of the Mercure.rocks Hub requires a transport to work.
Supported transports are Apache Pulsar, Apache Kafka and PostgreSQL.

#### Redis Transport

This is the recommended transport when the Hub isn't the main storage system and low latency is needed.
The Redis transport should fit most use cases.

To install Redis, [read the documentation](https://redis.io/topics/quickstart).
Most Cloud Computing platforms also provide managed versions of Redis.

| Feature         | Supported |
|-----------------|-----------|
| History         | ✅        |
| Presence API    | ✅        |
| Custom event ID | ✅        |

##### Configuration

All [the configuration parameters and formats](https://mercure.rocks/docs/hub/config) supported by the free Mercure.rocks Hub are also available.

To use Redis, the `MERCURE_TRANSPORT_URL` environment variable must be set like in this example:

    MERCURE_TRANSPORT_URL=redis://127.0.0.1:6379/mercure-ha \
    MERCURE_LICENSE=snip \
    MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    ./mercure run

The following options can be passed as query parameters of the URL set in `transport_url`:

| Parameter        | Description                                                                                            | Default |
|------------------|--------------------------------------------------------------------------------------------------------|---------|
| `tls`            | set to `1` to enable TLS support                                                                       | `0`     |
| `max_len_approx` | the approximative maximum number of messages to store in the history, set to `0` to store all messages | `0`     |


#### PostgreSQL Transport

The PostgreSQL Transport allows to store permanently the event and to query them using the full power of SQL.
It is mostly useful when using the Mercure.rocks Hub as an event store, or as a primary data store.

To install PostgreSQL, [read the documentation](https://www.postgresql.org/docs/12/tutorial-install.html).
Most Cloud Computing platforms also provide managed versions of PostgreSQL.

| Feature         | Supported    |
|-----------------|--------------|
| History         | ✅           |
| Presence API    | ❌ (planned) |
| Custom event ID | ✅           |

##### PostgreSQL Configuration

All [the configuration parameters and formats](https://mercure.rocks/docs/hub/config) supported by the free Mercure.rocks Hub are also available.

To use PostgreSQL `LISTEN`/`NOTIFY`, the `MERCURE_TRANSPORT_URL` environment variable must be set like in this example:

    MERCURE_TRANSPORT_URL=postgres://user:password@127.0.0.1/mercure-ha \
    MERCURE_LICENSE=snip \
    MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    ./mercure run

[Options supported by `libpq`](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING) can be passed as query parameters of the URL set in `transport_url`.

#### Kafka Transport

The Kafka transport should only be used when Kafka is already part of your stack.

To install Apache Kafka, [read the quickstart guide](https://kafka.apache.org/quickstart).

Most Cloud Computing platforms also provide managed versions of Kafka.
The Mercure.rocks hub has been tested with:

* Bitnami's Kafka Docker images (Kubernetes and the like)
* Amazon Managed Streaming for Apache Kafka (Amazon MSK)
* IBM Event Streams for IBM Cloud
* Heroku Kafka

| Feature         | Supported    |
|-----------------|--------------|
| History         | ✅           |
| Presence API    | ❌           |
| Custom event ID | ✅           |

##### Kafka Configuration

All [the configuration parameters and formats](https://mercure.rocks/docs/hub/config) supported by the free Mercure.rocks Hub are also available.

To use Kafka, the `MERCURE_TRANSPORT_URL` environment variable must be set like in this example:

    MERCURE_TRANSPORT_URL=kafka://kafka/?addr=localhost:9092&topic=mercure-ha \
    MERCURE_LICENSE=snip \
    MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    ./mercure run

The following options can be passed as query parameters of the URL set in `transport_url`:

| Parameter        | Description                                                                                                                                 |
|------------------|---------------------------------------------------------------------------------------------------------------------------------------------|
| `addr`           | addresses of the Kafka servers, you can pass several `addr` parameters to use several Kafka servers (ex: `addr=host1:9092&addr=host2:9092`) |
| `topic`          | the name of the Kafka topic to use (ex: `topic=mercure-ha`), **all Mercure.rocks hub instances must use the same topic**                          |
| `consumer_group` | consumer group, **must be different for every instance of the Mercure.rocks hub** (ex: `consumer_group=<random-string>`)                          |
| `user`           | Kafka SASL user (optional, ex: `user=kevin`)                                                                                                |
| `password`       | Kafka SASL password (optional, ex: `password=maman`)                                                                                        |
| `tls`            | Set to `1` to enable TLS (ex: `tls=1`)                                                                                                      |

#### Pulsar Transport

The Pulsar transport should only be used when Pulsar is already part of your stack.

To install Apache Pulsar, [read the documentation](https://pulsar.apache.org/docs/en/standalone/).

| Feature         | Supported    |
|-----------------|--------------|
| History         | ✅           |
| Presence API    | ❌           |
| Custom event ID | ❌ (planned) |

##### Pulsar Configuration

All [the configuration parameters and formats](https://mercure.rocks/docs/hub/config) supported by the free Mercure.rocks Hub are also available.

To use Pulsar, the `MERCURE_TRANSPORT_URL` environment variable must be set like in this example:

    MERCURE_TRANSPORT_URL=pulsar://localhost:6650?topic=mercure-ha&subscription_name=the-node-id \
    MERCURE_LICENSE=snip \
    MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    ./mercure run

The following options can be passed as query parameters of the URL set in `transport_url`:

| Parameters          | Description                                                                                                                                |   |
|---------------------|--------------------------------------------------------------------------------------------------------------------------------------------|---|
| `topic`             | the name of the Pulsar topic to use (ex: `topic=mercure`), **all Mercure.rocks hub instances must use the same topic**                           |   |
| `subscription_name` | the subscription name for this node, **must be different for every instance of the Mercure.rocks hub** (ex: `subscription_name=<random-string>`) |   |

### Docker Images and Kubernetes Chart

An official Docker image and [a Kubernetes Chart](install.md#kubernetes) are available.
Contact us if you need help to use them.

### Updates

New releases of the High Availability Mercure.rocks Hub are automatically available in the Amazon S3 bucket containing the binary and on the Docker registry.

## Support

For support requests related to the On Premise version of Mercure.rocks, send a mail to [contact@mercure.rocks](mailto:contact@mercure.rocks?subject=On%20Premise%20support%20request).
