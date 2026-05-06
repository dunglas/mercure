---
title: "Deploy the Mercure.rocks hub on Kubernetes with Helm"
description: "Install Mercure.rocks on Kubernetes with the official Helm chart, including SSE-aware probes, rolling updates, and rootless security context."
---

# Kubernetes

The official Helm chart is the path of least resistance.

```console
# Kubernetes
helm repo add mercure https://charts.mercure.rocks
helm install mercure mercure/mercure \
  --set publisherJwtKey='!ChangeThisMercureHubJWTSecretKey!' \
  --set subscriberJwtKey='!ChangeThisMercureHubJWTSecretKey!'
```

For real deployments, store the keys in a Kubernetes `Secret` and point the chart at it with `existingSecret` (the chart reads `publisher-jwt-key` and `subscriber-jwt-key` from the named secret) instead of passing keys on the command line.

Default values produce a single-replica deployment with BoltDB, a `ClusterIP` service, and SSE-aware rolling-update settings. The full list of values lives in the [chart README](https://github.com/dunglas/mercure/blob/main/charts/mercure/README.md).

## What the chart sets up for you

The defaults are tuned for SSE workloads, not generic web apps:

- `terminationGracePeriodSeconds: 660`: matches the 600s `write_timeout` plus margin so pods drain cleanly. See [Rolling updates](../production/rolling-updates.md).
- `strategy.rollingUpdate.maxSurge: 1, maxUnavailable: 0`: one replica rotates at a time without dropping capacity.
- `minReadySeconds: 30`: a newly-Ready replica gets time to warm its transport before the next rotation.

You don't have to know these to use the chart. You do have to know them if you change the chart's defaults.

## Production Helm values for the Mercure hub

The open-source chart defaults to a single replica with BoltDB. That's the right shape for the open-source build: BoltDB is local to each pod, so multi-replica setups require a shared transport (see the Self-Hosted block at the end of this page).

```yaml
# values.yaml
replicaCount: 1

# Read the JWT keys from a Kubernetes Secret you create separately.
# The Secret must contain "publisher-jwt-key" and "subscriber-jwt-key".
existingSecret: mercure-jwt

ingress:
  enabled: true
  hosts:
    - host: hub.example.com
      paths: ["/"]
  tls:
    - secretName: hub-tls
      hosts: [hub.example.com]

# Persist BoltDB and Caddy state
persistence:
  enabled: true
  size: 10Gi

resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 1
    memory: 1Gi

extraDirectives: |
  cors_origins https://app.example.com
  subscriptions
```

Create the JWT Secret once before installing the chart:

```console
# JWT Secret
kubectl create secret generic mercure-jwt \
  --from-literal=publisher-jwt-key='!ChangeThisMercureHubJWTSecretKey!' \
  --from-literal=subscriber-jwt-key='!ChangeThisMercureHubJWTSecretKey!'
```

For multi-replica deployments, use a transport that synchronizes between pods. The open-source build only supports BoltDB (single-node); for Redis, Postgres, Kafka, or Pulsar, [Self-Hosted Mercure](https://mercure.rocks/pricing) ships those transports. See [Multi-node and self-hosted](#multi-node-and-self-hosted) below.

> **Pro tip.** Running more than one replica with the open-source build is possible if every pod handles its own slice of the topics (sticky load balancing on a hash of the topic). It's fragile: losing a pod loses its history. The Self-Hosted Redis transport replaces that with a real cluster: any replica can serve any subscriber, and the history is centralized.

## Kubernetes probes for the Mercure hub

The Caddy admin API binds to `localhost:2019` for security. That means probes from outside the container (the standard `httpGet` form) can't reach it. Use `exec` probes:

```yaml
# Kubernetes Probes for the Mercure Hub
readinessProbe:
  exec:
    command:
      ["wget", "-q", "--spider", "http://localhost:2019/mercure/health/ready"]
  initialDelaySeconds: 10
  periodSeconds: 10
livenessProbe:
  exec:
    command:
      ["wget", "-q", "--spider", "http://localhost:2019/mercure/health/live"]
  initialDelaySeconds: 30
  periodSeconds: 30
```

The chart sets these by default.

If you really want `httpGet` probes, bind the admin API to all interfaces by adding `admin 0.0.0.0:2019` to the Caddyfile's global options. But that exposes `/stop`, `/load`, `/config`, and the rest of the admin API to the pod network. Generally not what you want.

See [Health monitoring](../production/health-monitoring.md) for what the probes actually check.

## Rootless Mercure on Kubernetes

Kubernetes runtimes (containerd 1.5+, cri-o) set `net.ipv4.ip_unprivileged_port_start=0` inside containers, so a non-root process can bind 80 and 443.

```yaml
# values.yaml (snippet)
podSecurityContext:
  runAsNonRoot: true
  runAsUser: 1000
  runAsGroup: 1000
  fsGroup: 1000
  seccompProfile:
    type: RuntimeDefault
securityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop: [ALL]
  readOnlyRootFilesystem: true
  runAsNonRoot: true
  runAsUser: 1000
```

The chart's volume layout (`/data`, `/config`, `/tmp` mounted writable) accommodates `readOnlyRootFilesystem: true`.

For older runtimes that haven't lowered `ip_unprivileged_port_start`, change the target port to an unprivileged value:

```yaml
# values.yaml (snippet)
service:
  port: 80
  targetPort: 8080
```

The Service still exposes 80 to the cluster.

## Scaling and SSE

A few things to know about scaling SSE in Kubernetes:

- **Horizontal scale only works with a multi-node transport.** With BoltDB, each pod has its own history. Subscribers connected to pod A don't see updates published to pod B.
- **HPAs based on CPU underestimate.** SSE is mostly waiting; CPU stays low while connection counts grow. Scale on `mercure_subscribers_connected` (Prometheus metric) instead.
- **Connection draining matters.** A 1-replica -> 5-replica scale-up is cheap. A 5 -> 1 scale-down kills 4/5 of your subscribers if you don't drain. The chart's `terminationGracePeriodSeconds` handles this; don't lower it.

## Configuring ingress for Mercure SSE

Two things SSE needs from your ingress:

1. **Don't buffer the response.** NGINX Ingress: `nginx.ingress.kubernetes.io/proxy-buffering: "off"`. Traefik does the right thing by default.
2. **Long read timeouts.** Default ingress timeouts (60s, 30s) close every SSE connection. Set them to several minutes.

NGINX Ingress example:

```yaml
# values.yaml (snippet)
ingress:
  annotations:
    nginx.ingress.kubernetes.io/proxy-buffering: "off"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
```

See [Reverse proxies](reverse-proxy.md) for full configurations.

## Upgrading the Mercure Helm release

```console
# Upgrading the Mercure Helm Release
helm repo update
helm upgrade mercure mercure/mercure -f values.yaml
```

The chart triggers a rolling update. Subscribers reconnect at the cadence set by `write_timeout`, distributed across the drain window; they don't all reconnect at once. See [Rolling updates](../production/rolling-updates.md) for the full mechanism.

## Multi-node and self-hosted

The chart supports the multi-node transports out of the box. Set `image.repository` to the Self-Hosted image and configure the transport block:

```yaml
# values.yaml
replicaCount: 3

image:
  repository: registry.mercure.rocks/mercure-enterprise
  tag: 1.0.0

license: "<your license key>"

extraDirectives: |
  transport redis {
    url rediss://default:p@ssw0rd@redis.example.com:6379
    stream mercure
  }
```

The license is checked in-process; no callback to a license server.

## Next steps for Mercure on Kubernetes

- [Configuration](configuration.md): directives and env vars.
- [Health monitoring](../production/health-monitoring.md): probes, metrics, dashboards.
- [Rolling updates](../production/rolling-updates.md): why the defaults are what they are.
- [High availability](../production/high-availability.md): when one node isn't enough.
