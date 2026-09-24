---
title: "Mercure high availability: Cloud and Enterprise on-premises"
description: "Scale Mercure with managed Cloud or Enterprise on your infrastructure. Configure Redis/Valkey, PostgreSQL, Kafka, and Pulsar for shared history and multi-node delivery."
---

# Mercure high availability

Build for your next traffic spike without rebuilding your real-time stack. Mercure can serve tens of thousands of connected users on one small server: a [published benchmark](https://speakerdeck.com/dunglas/2-plus-and-mercure?slide=41) reported **40,000 concurrent connections on an EC2 t3.micro** with the open-source hub and **200,000 with the on-premises HA version**. See [Load testing](load-testing.md) to measure your workload.

When availability matters, capacity is only part of the story. A second hub keeps your service reachable when a node fails or needs an upgrade. A shared transport carries updates and history across the cluster.

**[Choose Mercure Cloud](https://mercure.rocks/pricing) to let us operate the hub, or [Mercure Enterprise](https://mercure.rocks/pricing) to run a supported cluster on your own infrastructure.** Your applications keep using the same Mercure protocol.

## What the open-source build gives you

| Capability                    | Open-source                                                       |
| ----------------------------- | ----------------------------------------------------------------- |
| Concurrent connections        | Unlimited by the license; hardware-bound                          |
| Publish rate                  | Unlimited by the license; hardware-bound                          |
| History buffer                | No automatic size cap by default; disk-bound                      |
| Shared state across nodes     | Requires an [Enterprise transport](https://mercure.rocks/pricing) |
| Transports                    | BoltDB, local                                                     |
| TLS, HTTP/2, HTTP/3           | Included                                                          |
| JWT authorization, presence   | Included                                                          |
| Prometheus metrics, profiling | Included                                                          |

The open-source hub is ready for single-node production deployments. [Enterprise](https://mercure.rocks/pricing) adds shared transports and support when you need redundancy or horizontal scaling.

## When one node isn't enough

Use multiple nodes to survive host failures, distribute subscriber traffic, or place hubs closer to users. A load balancer alone does not synchronize hubs: with independent BoltDB instances, a publication sent to one node does not reach subscribers on another.

A shared transport connects the nodes. After a node fails, clients reconnect to another healthy node and request missed events from retained history. Established SSE connections cannot move between processes.

## The two paths beyond single-node

### Mercure Cloud (managed)

**Build your application. We'll run the hub.** [Mercure Cloud](https://mercure.rocks/pricing) includes managed hosting, automatic HTTPS, and custom domains. Pro plans and above include high availability. Choose a plan for your expected connections, publication rate, and message size.

[Start with Mercure Cloud](https://mercure.rocks/pricing).

### Self-hosted Mercure (multi-node, on your infrastructure)

**Mercure Enterprise brings clustering and maintainer support to your own servers.** Run it on Kubernetes, VMs, or bare metal, with Redis/Valkey, PostgreSQL, Kafka, or Pulsar as the shared backend. You choose where your data lives and how long to retain it.

Prefer us to operate it too? The **Managed On-Premise** option adds deployment, monitoring, and managed updates on your infrastructure. [Compare Self-Hosted plans](https://mercure.rocks/pricing) or [contact us](mailto:contact@mercure.rocks).

The licensed Docker image is `ghcr.io/dunglas/mercure-saas/mercure-saas:1.0`. After obtaining registry access and a license, use it in place of `dunglas/mercure`, set `MERCURE_LICENSE`, and configure a shared transport below. See [Kubernetes deployment](../deployment/kubernetes.md#multi-node-and-self-hosted) for Helm values.

## Self-hosted transports

The following modules are included in **[Mercure Enterprise](https://mercure.rocks/pricing)**. Add one transport block to your existing `mercure` configuration, keeping its issuer and authorization settings.

Put these examples directly in a custom Caddyfile: `{$VARIABLE}` is expanded before parsing. These transport modules do not expand runtime `{env.VARIABLE}` placeholders, and Caddy does not expand nested variables inside `MERCURE_EXTRA_DIRECTIVES`. For Helm, use the [Redis/Valkey storage configuration](../deployment/kubernetes.md#multi-node-and-self-hosted).

Replicas of one hub must share the same backend namespace. Independent hubs need different Redis/Valkey streams, Kafka/Pulsar topics, or PostgreSQL databases; separate PostgreSQL schemas are not sufficient. Set a distinct Mercure `name` for each independent hub in one Caddy process.

### Redis / Valkey

Redis and Valkey are a good starting point for a cluster, especially when you need presence across nodes. The transport uses streams for history and supports custom event IDs and the cluster-wide subscription API.

```caddyfile
mercure {
  transport redis {
    url {$REDIS_URL}
    stream mercure
    max_length 100000
  }
}
```

Set `REDIS_URL` to a Redis or Valkey URI, such as `rediss://default:password@redis.example.com:6379`. Use `redis://` for a connection without TLS. Store production credentials in your secret manager.

| Option                 | Description                                                                       |
| ---------------------- | --------------------------------------------------------------------------------- |
| `url`                  | Redis or Valkey connection URI.                                                   |
| `addresses`            | One or more `host:port` addresses, as an alternative to `url`.                    |
| `stream`               | Shared stream name. Default: `mercure`.                                           |
| `max_length`           | Approximate retention limit in entries. Default: `0` (unlimited).                 |
| `username`, `password` | Credentials when configuring addresses separately.                                |
| `tls`                  | Enable TLS when configuring addresses separately.                                 |
| `gob`                  | Encode updates with Go gob instead of JSON; configure every replica consistently. |

Configure backend persistence for your recovery requirements. Enable `subscriptions` on the hub to use the subscription API.

### PostgreSQL

Keep your real-time history in PostgreSQL, using infrastructure your team already knows. The transport persists updates in a `history` table and uses `LISTEN`/`NOTIFY` to signal new events to every hub.

```caddyfile
mercure {
  transport postgres {
    url {$POSTGRES_URL}
  }
}
```

Set `POSTGRES_URL` to a connection URI, for example `postgres://user:password@db.example.com/mercure?sslmode=require`. The database role must be able to create the transport's tables, functions, and triggers. The transport supports replay and custom event IDs; it does not implement the cluster-wide subscription API.

History is stored in SQL, so you can query it for diagnostics or analytics. Plan retention and backups as part of operating the database.

### Apache Kafka

Already running Kafka? Use it to distribute Mercure updates across your hubs and retain them under your broker's storage policy.

```caddyfile
mercure {
  transport kafka {
    addresses kafka-1:9092 kafka-2:9092
    topic mercure
    consumer_group {$HOSTNAME}
  }
}
```

| Option             | Description                                                                                                             |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------- |
| `addresses`        | One or more Kafka broker addresses.                                                                                     |
| `topic`            | Kafka topic shared by all replicas of the hub.                                                                          |
| `consumer_group`   | Unique per hub instance so every hub receives every update. Defaults to `HOSTNAME`; required if that variable is empty. |
| `user`, `password` | SASL credentials.                                                                                                       |
| `tls`              | Enable TLS to the brokers.                                                                                              |

Set a distinct `HOSTNAME` for each instance, as Kubernetes does for pods. Kafka supports replay and custom event IDs, but not the cluster-wide subscription API. Ordering is per Kafka partition; updates across partitions have no global order.

### Apache Pulsar

Connect Mercure to your Pulsar deployment to share publications and replay across hub instances:

```caddyfile
mercure {
  transport pulsar {
    url pulsar://pulsar.example.com:6650
    topic mercure
    subscription_name {$HOSTNAME}
  }
}
```

Use the same topic and a unique `subscription_name` per hub instance. The subscription name defaults to `HOSTNAME`, which must be set if the option is omitted. Configure broker retention for the replay window you need.

Pulsar supports history replay. It assigns the event IDs, so publisher-supplied custom IDs are not preserved. The cluster-wide subscription API is not supported.

## Picking a Mercure self-hosted transport

| Need                                            | Transport                |
| ----------------------------------------------- | ------------------------ |
| Shared history and cluster-wide presence        | **Redis / Valkey**       |
| SQL history on existing database infrastructure | **PostgreSQL**           |
| Use your Kafka cluster                          | **Kafka**                |
| Use your Pulsar cluster                         | **Pulsar**               |
| Single node with no separate backend            | **BoltDB** (open-source) |

Each Enterprise transport also accepts `liveness_threshold`, a duration controlling how long a backend outage can last before the hub reports a liveness failure. See [Health monitoring](health-monitoring.md).

For a first cluster, start with Redis/Valkey. For help choosing and sizing a deployment, [talk to the Mercure team](mailto:contact@mercure.rocks).

## Custom Mercure transports

The transport interface is small and public. If you need a custom backend, implement [`transport.go`](https://github.com/dunglas/mercure/blob/main/transport.go) and build a hub with `xcaddy`.

## License keys

Set `MERCURE_LICENSE` to the key supplied with your [Self-Hosted plan](https://mercure.rocks/pricing). Inject it through your deployment's secret store alongside the publisher and subscriber keys:

```console
export MERCURE_LICENSE='<your license key>'
./mercure run --config Caddyfile
```

The Enterprise binary validates the license signature and expiry locally at startup, without contacting a license server. A missing, invalid, or expired license prevents startup. Check the logs and contact support if validation fails.

## Mercure migration paths

The open-source hub, Cloud, and Enterprise speak the same protocol. You keep your publication and subscription logic when changing hosting.

Update the hub URL, token audience, issuer configuration, CORS origins, and cookie scope to match the destination. Plan history migration and client reconnection separately.

## Mercure vs. Pusher and Ably: pricing comparison

With Mercure, you choose how to pay for real-time delivery: managed Cloud, a supported Enterprise deployment on your infrastructure, or the free open-source hub. You can move between those options without adopting a new client SDK.

Evaluating **Pusher / Ably / Firebase / Supabase Realtime**? Mercure keeps your real-time delivery independent of your application database and gives you both managed and on-premises options. For large deployments, [Self-Hosted plans](https://mercure.rocks/pricing) let you size your own infrastructure; for teams that want to avoid operations, [Cloud plans](https://mercure.rocks/pricing) include hosting. See the [FAQ comparison](../reference/faq.md#whats-the-difference-between-mercure-and-pusher--ably--firebase--supabase-realtime).

## Mercure support channels

- **Enterprise / Cloud:** [contact@mercure.rocks](mailto:contact@mercure.rocks). [Choose a support plan](https://mercure.rocks/pricing) for your response-time and availability requirements.
- **Open-source:** [GitHub Discussions](https://github.com/dunglas/mercure/discussions), [Stack Overflow](https://stackoverflow.com/questions/tagged/mercure), or the [Symfony Slack](https://symfony.com/slack).

## Next steps

- [Kubernetes](../deployment/kubernetes.md#multi-node-and-self-hosted): deploy the Enterprise image with Helm.
- [Rolling updates](rolling-updates.md): drain connections during deployments.
- [Health monitoring](health-monitoring.md): monitor your hub and shared transport.
