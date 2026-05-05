---
title: "Load Testing the Mercure.rocks Hub with Gatling"
description: "Run the Gatling-based Mercure load test to measure subscriber capacity, publish throughput, and identify file-descriptor and matcher bottlenecks."
---

# Load Testing

The Mercure repository ships a [Gatling](https://gatling.io)-based load test. Use it to measure your own infrastructure before users do.

For reference, a public benchmark by Glory4Gamers reached **40,000 concurrent connections on a single EC2 t3.micro** running the open-source hub. Your numbers will vary with kernel limits, NIC, and publish rate. Don't take 40k as a ceiling; take it as "one node holds a lot."

## Run the Mercure Gatling Load Test

```console
# Run the Mercure Gatling Load Test
git clone https://github.com/dunglas/mercure
cd mercure/gatling
./mvnw gatling:test
```

Without configuration, the test hits a local hub on `https://localhost`. To target a real hub, set `HUB_URL` and a publisher JWT.

## Mercure Load Test Configuration

All variables are optional.

| Variable                                        | Description                                                                               |
| ----------------------------------------------- | ----------------------------------------------------------------------------------------- |
| `HUB_URL`                                       | URL of the hub to test.                                                                   |
| `JWT`                                           | Publisher JWT.                                                                            |
| `SUBSCRIBER_JWT`                                | Subscriber JWT. Falls back to `JWT` when private updates are tested.                      |
| `INITIAL_SUBSCRIBERS`                           | Concurrent subscribers connected at the start.                                            |
| `SUBSCRIBERS_RATE_FROM` / `SUBSCRIBERS_RATE_TO` | Range for additional subscriber connection rate (per second).                             |
| `PUBLISHERS_RATE_FROM` / `PUBLISHERS_RATE_TO`   | Range for publication rate (per second).                                                  |
| `INJECTION_DURATION`                            | How long the publisher load runs.                                                         |
| `CONNECTION_DURATION`                           | How long subscribers stay connected.                                                      |
| `RANDOM_CONNECTION_DURATION`                    | Randomize subscriber lifetime up to `CONNECTION_DURATION`.                                |
| `PRIVATE_UPDATES`                               | If set, send private updates with random topics instead of public updates with one topic. |

A useful starting recipe (build up to your expected traffic):

```console
# Mercure Load Test Configuration
HUB_URL=https://hub.example.com/.well-known/mercure \
JWT=<publisher JWT> \
INITIAL_SUBSCRIBERS=1000 \
SUBSCRIBERS_RATE_FROM=50 \
SUBSCRIBERS_RATE_TO=200 \
PUBLISHERS_RATE_FROM=10 \
PUBLISHERS_RATE_TO=100 \
INJECTION_DURATION=300 \
CONNECTION_DURATION=600 \
./mvnw gatling:test
```

## What to Measure During a Mercure Load Test

While the test runs, watch:

- **`mercure_subscribers_connected`**: should track the configured ramp.
- **CPU and memory** of the hub process: establishes the per-subscriber cost on your hardware.
- **Open file descriptors** (`ls /proc/<pid>/fd | wc -l`): every subscriber takes one. Compare to your `ulimit -n`.
- **Publish latency**: Caddy request duration histogram on `POST /.well-known/mercure`.
- **Subscriber receive latency**: built into the Gatling report.

## What Changes the Numbers

Connections themselves are cheap. What scales the cost:

- **Publish rate x number of matching subscribers per topic.** A 1-publish-per-second feed to 100k subscribers is far heavier than 1k publishes per second to 100 subscribers each.
- **Dispatch timeout.** Slow subscribers blocking dispatch eat goroutines until `dispatch_timeout` cuts them off.
- **Matcher complexity.** Exact matchers are O(1); regex and CEL matchers cost time per evaluation. Use `topic_selector_cache` for repeated patterns.
- **History writes.** BoltDB syncs to disk; write throughput is bounded by your storage. The Postgres transport is faster on bursty writes; Redis is the fastest.

## Common Mercure Hub Bottlenecks

| Symptom                               | Probable cause                                                                                                |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| `accept: too many open files` in logs | `ulimit -n` too low. Set `100000` or higher on the host.                                                      |
| CPU spent in matcher evaluation       | Regex/CEL matchers; raise `topic_selector_cache`.                                                             |
| Dispatch latency rising under load    | Slow subscribers; lower `dispatch_timeout` to bound the impact.                                               |
| Memory growth that doesn't plateau    | Goroutine leak; capture a `pprof` heap and goroutine snapshot ([Debugging](debugging.md)) and file an issue.  |
| Test plateaus before the box does     | Backpressure from the hub's listener; check `net.core.somaxconn` and `net.ipv4.tcp_max_syn_backlog` on Linux. |

## File Descriptor Limits for the Mercure Hub

The single most common limit. On Linux:

```console
# Per process (the running hub)
prlimit --pid $(pgrep mercure)

# Set globally for the next process you start
ulimit -n 100000

# Persist via systemd
# /etc/systemd/system/mercure.service.d/override.conf
[Service]
LimitNOFILE=100000
```

In Docker:

```yaml
# File Descriptor Limits for the Mercure Hub
services:
  mercure:
    ulimits:
      nofile:
        soft: 100000
        hard: 100000
```

In Kubernetes, the host's limit applies to the container by default. If the host is set to `1024`, that's your ceiling. Bump it on the node.

## Mercure Conformance vs. Load Testing

Load tests measure throughput. Conformance tests check correctness. Run both:

- [Load test](load-testing.md) (this page).
- [Conformance tests](../ecosystem/conformance-tests.md): Playwright suite that verifies the hub follows the protocol.

## When to Scale Beyond One Node

Symptoms that mean a single node won't get you any further:

- CPU pinned at 100% during normal traffic, no headroom for spikes.
- Network bandwidth saturated by fan-out (publish x subscribers per topic exceeds what your NIC can deliver).
- You need geographic redundancy, not just headroom.

At that point: [High availability](high-availability.md).

## Next Steps for Mercure Load Testing

- [Debugging](debugging.md): `pprof` for figuring out where the time goes.
- [Health monitoring](health-monitoring.md): what to watch in steady state.
- [High availability](high-availability.md): multi-node options.
