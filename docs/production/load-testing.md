---
title: "Load testing the Mercure.rocks hub with Gatling"
description: "Run the Gatling-based Mercure load test to measure subscriber capacity, publish throughput, and identify file-descriptor and matcher bottlenecks."
---

# Mercure load testing

The Mercure repository ships a [Gatling](https://gatling.io)-based load test. Use it to measure your own infrastructure before users do.

A [published Mercure benchmark](https://speakerdeck.com/dunglas/2-plus-and-mercure?slide=41) reported **40,000 concurrent connections on an EC2 t3.micro** with the open-source hub and **200,000 with the on-premises HA version**. These results show how far Mercure can scale; use the test below to size your own deployment.

Capacity depends on publication rate, fan-out, payload size, history writes, and network limits. Measure with representative traffic; an idle-connection benchmark does not establish publish throughput.

## Run the Mercure Gatling load test

```console
git clone https://github.com/dunglas/mercure
cd mercure/gatling
./mvnw gatling:test
```

Without configuration, the test hits a local hub on `https://localhost`. To target a real hub, set `HUB_URL` and a publisher JWT.

## Mercure load test configuration

All variables are optional.

| Variable                                        | Description                                                                                   |
| ----------------------------------------------- | --------------------------------------------------------------------------------------------- |
| `HUB_URL`                                       | URL of the hub to test.                                                                       |
| `JWT`                                           | Publisher JWT.                                                                                |
| `SUBSCRIBER_JWT`                                | Subscriber JWT. Falls back to `JWT` when private updates are tested.                          |
| `INITIAL_SUBSCRIBERS`                           | Concurrent subscribers connected at the start.                                                |
| `SUBSCRIBERS_RATE_FROM` / `SUBSCRIBERS_RATE_TO` | Range for additional subscriber connection rate (per second).                                 |
| `PUBLISHERS_RATE_FROM` / `PUBLISHERS_RATE_TO`   | Range for publication rate (per second).                                                      |
| `INJECTION_DURATION`                            | Subscriber injection duration in seconds; publishers run for this plus `CONNECTION_DURATION`. |
| `CONNECTION_DURATION`                           | How long subscribers stay connected.                                                          |
| `RANDOM_CONNECTION_DURATION`                    | Boolean (`true` by default); randomize subscriber lifetime below `CONNECTION_DURATION`.       |
| `PRIVATE_UPDATES`                               | Set `true` for private updates; defaults to `false`.                                          |

Start below your expected traffic and increase the load while measuring latency and errors:

```console
HUB_URL=https://hub.example.com/.well-known/mercure \
JWT='<publisher JWT>' \
INITIAL_SUBSCRIBERS=1000 \
SUBSCRIBERS_RATE_FROM=50 \
SUBSCRIBERS_RATE_TO=200 \
PUBLISHERS_RATE_FROM=10 \
PUBLISHERS_RATE_TO=100 \
INJECTION_DURATION=300 \
CONNECTION_DURATION=600 \
./mvnw gatling:test
```

## What to measure during a Mercure load test

While the test runs, watch:

- **`mercure_subscribers_connected`**: should track the configured ramp.
- **CPU and memory** of the hub process: establishes the per-subscriber cost on your hardware.
- **Open file descriptors** (`ls /proc/<pid>/fd | wc -l`): each TCP connection takes one; multiplexed streams share it. Compare to your `ulimit -n`.
- **Publish latency**: Caddy request duration histogram on `POST /.well-known/mercure`.
- **Subscriber receive latency**: built into the Gatling report.

## What changes the numbers

Connections themselves are cheap. What scales the cost:

- **Publish rate x number of matching subscribers per topic.** Both 1 publish/second to 100,000 subscribers and 1,000 publishes/second to 100 subscribers produce 100,000 deliveries/second, but their scheduling and storage costs differ.
- **Dispatch timeout.** Slow subscribers blocking dispatch eat goroutines until `dispatch_timeout` cuts them off.
- **Matcher complexity.** String length and URL Pattern complexity affect matching cost. Use `topic_matcher_cache` for repeated patterns.
- **History writes.** BoltDB syncs to disk; write throughput is bounded by your storage. Benchmark alternative transports with your durability and retention settings.

## Common Mercure hub bottlenecks

| Symptom                               | Probable cause                                                                                                                     |
| ------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `accept: too many open files` in logs | `ulimit -n` too low. Set `100000` or higher on the host.                                                                           |
| CPU spent in matcher evaluation       | URL Pattern matchers; raise `topic_matcher_cache`.                                                                                 |
| Dispatch latency rising under load    | Slow subscribers; lower `dispatch_timeout` to bound the impact.                                                                    |
| Memory growth that doesn't plateau    | Possible retained data or goroutines; capture a `pprof` heap and goroutine snapshot ([Debugging](debugging.md)) and file an issue. |
| Test plateaus before the box does     | Backpressure from the hub's listener; check `net.core.somaxconn` and `net.ipv4.tcp_max_syn_backlog` on Linux.                      |

## File descriptor limits for the Mercure hub

The single most common limit. On Linux:

```console
prlimit --pid $(pgrep mercure)

# Set the limit for this shell and its child processes
ulimit -n 100000

# Persist via systemd
# /etc/systemd/system/mercure.service.d/override.conf
[Service]
LimitNOFILE=100000
```

In Docker:

```yaml
services:
  mercure:
    ulimits:
      nofile:
        soft: 100000
        hard: 100000
```

In Kubernetes, inspect the running container's file-descriptor limit. Configure the node or container runtime if it is too low.

## Mercure conformance vs. Load testing

Load tests measure throughput. Conformance tests check correctness. Run both:

- [Load test](load-testing.md) (this page).
- [Conformance tests](../ecosystem/conformance-tests.md): Playwright suite that verifies the hub follows the protocol.

## When to scale beyond one node

Symptoms that mean a single node won't get you any further:

- CPU pinned at 100% during normal traffic, no headroom for spikes.
- Network bandwidth saturated by fan-out (publish x subscribers per topic exceeds what your NIC can deliver).
- You need geographic redundancy, not just headroom.

At that point: [High availability](high-availability.md).

## Next steps

- [Debugging](debugging.md): `pprof` for figuring out where the time goes.
- [Health monitoring](health-monitoring.md): what to watch in steady state.
- [High availability](high-availability.md): multi-node options.
