---
title: "Deploy the Mercure.rocks hub on Kubernetes with Helm"
description: "Install Mercure.rocks on Kubernetes with the official Helm chart, including SSE-aware probes, rolling updates, and rootless security context."
---

# Deploy Mercure on Kubernetes

**Run a supported Mercure cluster on your infrastructure with [Mercure Enterprise](https://mercure.rocks/pricing).** The chart supports its Redis/Valkey, PostgreSQL, Kafka, and Pulsar transports. Our Managed On-Premise option covers deployment and operation; [Mercure Cloud](https://mercure.rocks/pricing) lets you skip Kubernetes entirely.

Install the Mercure hub with the official Helm chart. Generate each key once with `openssl rand -base64 32` and export it as `MERCURE_PUBLISHER_JWT_KEY` / `MERCURE_SUBSCRIBER_JWT_KEY`: anyone can forge tokens signed with the development key.

```console
helm repo add mercure https://charts.mercure.rocks
helm install mercure mercure/mercure \
  --set publisherJwtKey="$MERCURE_PUBLISHER_JWT_KEY" \
  --set subscriberJwtKey="$MERCURE_SUBSCRIBER_JWT_KEY"
```

For production, use `existingSecret` with a Secret containing all the chart's required keys, as shown below. Setting `existingSecret` disables creation of the chart-managed Secret, including its extra directives.

Default values produce a single-replica deployment with BoltDB, a `ClusterIP` service, and SSE-aware rolling-update settings. The full list of values lives in the [chart documentation](https://github.com/dunglas/mercure/blob/main/charts/mercure/README.md).

## What the chart sets up for you

With the default `RollingUpdate` strategy, the chart sets:

- `terminationGracePeriodSeconds: 660`: matches the 600s `write_timeout` plus margin so pods drain cleanly. See [Rolling updates](../production/rolling-updates.md).
- `strategy.rollingUpdate.maxSurge: 1, maxUnavailable: 0`: one replica rotates at a time without dropping capacity.
- `minReadySeconds: 30`: a newly-Ready replica gets time to warm its transport before the next rotation.

Keep these values aligned with `write_timeout` when changing the deployment.

## Production Helm values for the Mercure hub

Save the following as `values.yaml`. Use one replica for persistent BoltDB. Multiple replicas serving the same topics require an [Enterprise shared transport](../production/high-availability.md#self-hosted-transports).

```yaml
replicaCount: 1
updateStrategy:
  type: Recreate

# Create the complete Secret shown below before installing.
existingSecret: mercure-jwt

ingress:
  enabled: true
  hosts:
    - host: hub.example.com
      paths:
        - path: /
          pathType: Prefix
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

extraEnvs:
  - name: MERCURE_TRUSTED_ISSUERS
    value: https://app.example.com
```

With persistent BoltDB, `Recreate` prevents two pods from opening the same database. This causes a service interruption during upgrades. The chart applies its extended drain and readiness settings only to `RollingUpdate`; use a shared transport for rolling upgrades across replicas.

Create `mercure-secret.yaml` with the following keys. Replace the example secrets and hostnames, then apply it before installing the chart:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mercure-jwt
type: Opaque
stringData:
  publisher-jwt-key: "<output of openssl rand -base64 32>"
  subscriber-jwt-key: "<output of openssl rand -base64 32>"
  extra-directives: |
    cors_origins https://app.example.com
    subscriptions
    resource_identifier https://hub.example.com/.well-known/mercure
  license: ""
  caddy-extra-config: ""
  caddy-extra-directives: ""
```

```console
kubectl apply -f mercure-secret.yaml
helm upgrade --install mercure mercure/mercure -f values.yaml
```

Keep the Secret manifest in your secret-management workflow. With `existingSecret`, place `extra-directives` in that Secret; the `extraDirectives` Helm value is not used.

Multiple replicas need a shared transport. BoltDB and the local transport do not synchronize updates across pods. See [Multi-node and self-hosted](#multi-node-and-self-hosted).

## Kubernetes probes for the Mercure hub

The Caddy admin API binds to `localhost:2019` for security. That means probes from outside the container (the standard `httpGet` form) can't reach it. Use `exec` probes:

```yaml
readinessProbe:
  exec:
    command:
      [
        "wget",
        "-q",
        "-O",
        "/dev/null",
        "http://localhost:2019/mercure/health/ready",
      ]
  initialDelaySeconds: 10
  periodSeconds: 10
livenessProbe:
  exec:
    command:
      [
        "wget",
        "-q",
        "-O",
        "/dev/null",
        "http://localhost:2019/mercure/health/live",
      ]
  initialDelaySeconds: 30
  periodSeconds: 30
```

The chart sets these by default.

If you really want `httpGet` probes, bind the admin API to all interfaces by adding `admin 0.0.0.0:2019` to the Caddyfile's global options. But that exposes `/stop`, `/load`, `/config`, and the rest of the admin API to the pod network. Generally not what you want.

See [Health monitoring](../production/health-monitoring.md) for what the probes actually check.

## Rootless Mercure on Kubernetes

Binding to ports below 1024 as a non-root user depends on the runtime's `net.ipv4.ip_unprivileged_port_start` setting. Use the security context below, and select port 8080 if your runtime requires it.

```yaml
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
2. **Long read timeouts.** Keep the ingress idle timeout above the heartbeat interval, with margin for delays.

NGINX Ingress example:

```yaml
ingress:
  annotations:
    nginx.ingress.kubernetes.io/proxy-buffering: "off"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
```

See [Reverse proxies](reverse-proxy.md) for full configurations.

## Upgrading the Mercure Helm release

```console
helm repo update
helm upgrade mercure mercure/mercure -f values.yaml
```

The chart updates the deployment according to its strategy. With a shared transport, subscribers can reconnect to another ready replica while the old one drains. See [Rolling updates](../production/rolling-updates.md) for the full mechanism.

## Multi-node and self-hosted

The chart supports [Mercure Enterprise](https://mercure.rocks/pricing) out of the box. Add the following to a separate set of production values for a Redis or Valkey cluster. Use the licensed image and configure the transport:

```yaml
replicaCount: 3
updateStrategy:
  type: RollingUpdate

persistence:
  enabled: false

image:
  repository: ghcr.io/dunglas/mercure-saas/mercure-saas
  tag: "1.0"

license: "<your license key>"

extraDirectives: |
  transport redis {
    address caddy-storage-redis.alt
    stream mercure
  }
```

[Obtain a Self-Hosted license](https://mercure.rocks/pricing) and registry access, then configure `imagePullSecrets` for GHCR. Add the [Redis/Valkey storage settings below](#storing-the-redis-or-valkey-password-securely) to these values. `address caddy-storage-redis.alt` reuses that connection for the Mercure transport. Configure persistence on the shared backend; the hub pods no longer need BoltDB volumes.

Configure your JWT keys, trusted issuer, public resource identifier, and CORS as in the single-node example. If using `existingSecret`, put the license and transport block in its `license` and `extra-directives` keys instead of the `license` and `extraDirectives` Helm values. See [Enterprise transport configuration](../production/high-availability.md#self-hosted-transports) for PostgreSQL, Kafka, and Pulsar.

## Storing the Redis or Valkey password securely

The chart writes `globalOptions` into a ConfigMap (`templates/configmap.yaml`), while `extraDirectives`, the JWT keys, and the license live in a Secret. A `storage redis { ... password "..." ... }` block placed in `globalOptions` therefore exposes the password to anyone with `get configmap` on the namespace.

Keep the password out of the ConfigMap with Caddy's `{env.NAME}` placeholder, sourcing the value from a Secret:

```console
kubectl create secret generic mercure-redis \
  --from-literal=password='<your Redis password>'
```

```yaml
globalOptions: |
  storage redis {
      host redis.example.com
      port 6380
      username default
      password "{env.REDIS_PASSWORD}"
      tls_enabled true
  }
extraEnvs:
  - name: REDIS_PASSWORD
    valueFrom:
      secretKeyRef:
        name: mercure-redis
        key: password
```

The bundled `caddy-storage-redis` module expands `{env.REDIS_PASSWORD}` when it loads the configuration. The ConfigMap contains only the placeholder, and the Mercure transport reuses the authenticated storage client. The Mercure transport modules themselves do not expand runtime `{env.*}` placeholders.

Env vars sourced from a Secret are injected at pod start: updating the Secret does not reach running pods. Rotate the password with `kubectl rollout restart deployment/mercure` or a secret-reloader controller.

## Next steps

- [Configuration](configuration.md): directives and env vars.
- [Health monitoring](../production/health-monitoring.md): probes, metrics, dashboards.
- [Rolling updates](../production/rolling-updates.md): why the defaults are what they are.
- [High availability](../production/high-availability.md): when one node isn't enough.
