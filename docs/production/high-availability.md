---
title: "Mercure High Availability and Self-Hosted Multi-Node Transports"
description: "Scale Mercure beyond a single node with the Self-Hosted Redis, PostgreSQL, Kafka, or Pulsar transports, or with managed Mercure Cloud."
---

# High Availability

The open-source Mercure hub is a serious piece of software — a single instance comfortably handles tens of thousands of concurrent connections on modest hardware ([benchmark: 40k concurrent on a t3.micro](load-testing.md)). For most production workloads, **one node is enough**.

What one node can't give you is redundancy. If the box goes down, every subscriber reconnects to nothing. If the disk fails, the BoltDB history is gone. If you want to survive that, you need more than one node — and more than one node needs a transport that synchronizes between them.

This page covers your options.

## What the Open-Source Build Gives You

| Capability | Open-source |
| --- | --- |
| Concurrent connections | **Unlimited** (hardware-bound) |
| Publish rate | **Unlimited** |
| History buffer | **Unlimited** (disk-bound) |
| Number of nodes | 1 |
| Transports | BoltDB, local |
| TLS, HTTP/2, HTTP/3 | Yes |
| Authorization | Full JWT support |
| Metrics, profiling | Full Prometheus + pprof |
| Subscription events | Yes |

The "1 node" line is the only ceiling. Everything else is unbounded by the license — only by what your hardware and network can deliver.

## When One Node Isn't Enough

Three reasons people graduate to multi-node:

1. **Redundancy.** A single replica is a single point of failure. For real-time SLOs (sub-second delivery, no perceptible reconnect), you need more than one replica.
2. **Throughput beyond a single host.** A box can usually push as much as its NIC allows, but multi-host gives you horizontal scale for fan-out: 1M-subscriber broadcasts split across nodes.
3. **Geo-distribution.** Multi-region deployments need a transport that crosses regions cheaply.

Connection counts alone rarely justify multi-node — a single hub at 100k concurrent connections is normal.

## The Two Paths Beyond Single-Node

### Mercure Cloud (Managed)

A hub provisioned on the [Mercure.rocks Cloud](https://mercure.rocks/pricing). High-availability infrastructure, TLS, custom domains, SRE on call. You don't run anything.

| Tier | €/month | Connections | History |
| --- | --- | --- | --- |
| Free | 0 | 25 | None |
| Hobby | 35 | 1,000 | 100 messages |
| Pro | 120 | 5,000 | 500 messages |
| Business | 450 | 20,000 | 5,000 messages |

The buffer caps exist because managed hubs need predictable storage. If you need more history per topic, run Self-Hosted instead.

The protocol is identical. Migrate later by changing one URL.

### Self-Hosted Mercure (Multi-Node, on Your Infrastructure)

A licensed build of the same hub with multi-node transports added. You run it on your servers (bare metal, your own Kubernetes, your own clouds). Data never leaves your infrastructure — useful for GDPR data residency, HIPAA, internal compliance.

| Tier | €/year | Connections | Nodes | History | Support |
| --- | --- | --- | --- | --- | --- |
| Open Source | 0 | Unlimited | 1 | Unlimited | Community |
| Startup | 1,500 | 1,000 | 2 | Unlimited | Email |
| Business | 5,000 | 10,000 | 3 | Unlimited | Priority next-day |
| Corporate | 12,000 | Unlimited | Unlimited | Unlimited | Priority + SLA |
| Elite | Custom | Unlimited | Unlimited | Unlimited | 24/7 + SLA |

A separate **Managed On-Premise** add-on (€5,000/year) covers remote setup, monitoring, and managed updates if you want the binaries on your infra without running them yourself.

To purchase, email [contact@mercure.rocks](mailto:contact@mercure.rocks?subject=Self-Hosted%20Mercure).

## Self-Hosted Transports

### Redis / Valkey

The default for low-latency multi-node. Good fit when the hub is one of several services and the data is volatile.

```caddyfile
# Redis / Valkey
mercure {
  transport redis {
    url    rediss://default:p@ssw0rd@redis.example.com:6379
    stream mercure
  }
  # ...
}
```

| Feature | Supported |
| --- | --- |
| History | ✅ |
| Subscription API | ✅ |
| Custom event ID | ✅ |

Options:

| Option | Description |
| --- | --- |
| `url` | Redis connection URI ([spec](https://github.com/redis/redis-specifications/blob/master/uri/redis.txt)). |
| `stream` | Redis stream name. Default: `mercure`. |
| `max_length` | Approximate maximum stream size. `0` for unlimited. |
| `gob` | Use Go `gob` instead of JSON. Faster, but can't be read by other clients. |
| `addresses` | Multiple Redis nodes (cluster). |
| `username`, `password`, `tls` | Authentication and transport security. |

Reuse an existing Caddy storage Redis (when you also use [`caddy-storage-redis`](https://github.com/pberkel/caddy-storage-redis)) by passing `address caddy-storage-redis.alt`.

### PostgreSQL

The Postgres transport uses `LISTEN`/`NOTIFY` for pub/sub and SQL tables for history. Right when you want events queryable from the rest of your data.

```caddyfile
# PostgreSQL
mercure {
  transport postgres {
    url postgres://user:password@db.example.com/mercure
  }
}
```

| Feature | Supported |
| --- | --- |
| History | ✅ |
| Subscription API | ❌ (planned) |
| Custom event ID | ✅ |

The Postgres transport doubles as an event store. You can join Mercure events with your application data in a single query — useful for audit, analytics, and replays.

### Apache Kafka

Use Kafka when it's already in your stack and you want Mercure to ride on it. Otherwise, prefer Redis or Postgres.

```caddyfile
# Apache Kafka
mercure {
  transport kafka {
    addresses host1:9092 host2:9092
    topic mercure
    consumer_group hub-pod-3
  }
}
```

| Option | Description |
| --- | --- |
| `addresses` | Broker addresses. |
| `topic` | Kafka topic. **All hub instances must share the same topic.** |
| `consumer_group` | Consumer group. **Must be unique per hub instance.** |
| `user`, `password`, `tls` | SASL credentials. |

| Feature | Supported |
| --- | --- |
| History | ✅ |
| Subscription API | ❌ |
| Custom event ID | ✅ |

### Apache Pulsar

```caddyfile
# Apache Pulsar
mercure {
  transport pulsar {
    url pulsar://pulsar.example.com:6650
    topic mercure
    subscription_name hub-pod-3
  }
}
```

| Feature | Supported |
| --- | --- |
| History | ✅ |
| Subscription API | ❌ |
| Custom event ID | ❌ (planned) |

## Picking a Mercure Self-Hosted Transport

| Need | Transport |
| --- | --- |
| Lowest latency, simplest setup | **Redis / Valkey** |
| Queryable history alongside app data | **PostgreSQL** |
| Already running Kafka | **Kafka** |
| Already running Pulsar | **Pulsar** |
| Single node, no extra infra | **BoltDB** (open-source) |

When in doubt, Redis. It's the recommended default for Self-Hosted.

## Custom Mercure Transports

The transport interface is small and public. If none of the above fits, write your own — see [`transport.go`](https://github.com/dunglas/mercure/blob/main/transport.go) and build a custom hub with `xcaddy`.

## License Keys

Self-Hosted is gated by a license key passed via `MERCURE_LICENSE`. The check runs in-process; the hub doesn't call back to a license server.

```console
# License keys
MERCURE_LICENSE=<key> \
MERCURE_PUBLISHER_JWT_KEY=... \
MERCURE_SUBSCRIBER_JWT_KEY=... \
./mercure run
```

The license enforces node count and connection caps. Going over the cap doesn't crash the hub; it returns `429 Too Many Requests` to publishers and refuses new subscribers.

## Mercure Migration Paths

| From | To | What changes |
| --- | --- | --- |
| Open-source single node | Cloud | Change the hub URL on clients. JWT keys move to the dashboard. |
| Open-source single node | Self-Hosted | Same binary structure, with a license and a multi-node transport. Subscribe and publish APIs are byte-for-byte the same. |
| Cloud | Self-Hosted | Migrate the hub URL and the keys. Keep the same JWTs. |

There is no protocol fork: every tier speaks the same Mercure protocol. Code written against the open-source hub runs on Cloud and Self-Hosted unchanged.

## Mercure vs. Pusher and Ably: Pricing Comparison

For people coming from SaaS-only real-time platforms, here's the rough picture:

| Feature | Mercure Pro (€120) | Pusher Business ($499) | Ably Pro ($399+) |
| --- | --- | --- | --- |
| Concurrent connections | 5,000 | 2,000 | 5,000 |
| History buffer | 500 messages | Limited | 2 minutes default |
| Messages | Unlimited | Daily cap | Usage-based billing |
| Self-hostable? | Yes | No | No |

Mercure is the only one of these you can run on your own infrastructure if you need to. That's by design.

## Mercure Support Channels

- **Self-Hosted / Cloud:** [contact@mercure.rocks](mailto:contact@mercure.rocks)
- **Open-source:** [GitHub Discussions](https://github.com/dunglas/mercure/discussions), [Stack Overflow `mercure` tag](https://stackoverflow.com/questions/tagged/mercure), `#mercure` on the Symfony Slack

## Next Steps for Mercure High Availability

- [Rolling updates](rolling-updates.md) — graceful drain in any deployment.
- [Health monitoring](health-monitoring.md) — knowing the hub is healthy.
- [Load testing](load-testing.md) — figure out what the hardware can do before users do.
