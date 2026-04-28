<!-- markdownlint-disable -->
# Mercure Chart for Kubernetes

![Version: 0.23.2](https://img.shields.io/badge/Version-0.23.2-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.23.2](https://img.shields.io/badge/AppVersion-v0.23.2-informational?style=flat-square)

A Helm chart to install a Mercure Hub in a Kubernetes cluster. Mercure is a protocol to push data updates to web browsers and other HTTP clients in a convenient, fast, reliable and battery-efficient way.

[Learn more about Mercure.](https://mercure.rocks)

## Installing the Chart

To install the chart with the release name `my-release`, run the following commands:

    helm repo add mercure https://charts.mercure.rocks
    helm install my-release mercure/mercure

## Requirements

Kubernetes: `>=1.23.0-0`

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| adminPort | int | `2019` | Port used for the Caddy admin API (health checks, metrics, graceful shutdown). |
| affinity | object | `{}` | [Affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) configuration. See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#scheduling) for details. |
| autoscaling | object | Disabled by default. | Autoscaling must not be enabled unless you are using [the High Availability version](https://mercure.rocks/docs/hub/cluster) (see [values.yaml](values.yaml) for details). |
| autoscaling.behavior | object | `{}` | [Scaling policies](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#configurable-scaling-behavior) passed to the HPA `spec.behavior`. |
| autoscaling.customMetrics | list | `[]` | Additional metrics appended to the HPA `spec.metrics` list (Pods, Object, External metric types). |
| caddyExtraConfig | string | `""` | Inject snippet or named-routes options in the Caddyfile |
| caddyExtraDirectives | string | `""` | Inject extra Caddy directives in the Caddyfile. |
| ciliumNetworkPolicy | object | Disabled by default. | [CiliumNetworkPolicy](https://docs.cilium.io/en/stable/security/policy/) for the hub pods. Use this on Cilium-enabled clusters when you need features that the standard NetworkPolicy spec does not support (FQDN-based egress, L7 rules, explicit deny rules, etc.). Independent of `networkPolicy.enabled` — enable whichever your CNI supports. |
| ciliumNetworkPolicy.egress | list | `[]` | Allowed outbound traffic. Pass-through to `spec.egress`. The DNS rule below is required when using `toFQDNs`: Cilium learns IPs by inspecting responses, so DNS visibility on `kube-dns` must be allowed first. |
| ciliumNetworkPolicy.egressDeny | list | `[]` | Explicit outbound deny rules. Pass-through to `spec.egressDeny`. |
| ciliumNetworkPolicy.enabled | bool | `false` | Enable the CiliumNetworkPolicy. Requires the `cilium.io/v2` CRDs to be installed in the cluster. |
| ciliumNetworkPolicy.ingress | list | `[]` | Allowed inbound traffic. Pass-through to `spec.ingress`. |
| ciliumNetworkPolicy.ingressDeny | list | `[]` | Explicit inbound deny rules. Pass-through to `spec.ingressDeny`. |
| deployment.annotations | object | `{}` | Annotations to be added to the deployment. |
| dev | bool | `false` | Enable the development mode, including the debug UI and the demo. |
| existingSecret | string | `""` | Allows to pass an existing secret name, the above values will be used if empty. |
| extraDirectives | string | `""` | Inject extra Mercure directives in the Caddyfile. |
| extraEnvs | list | `[]` | Additional environment variables to set |
| fullnameOverride | string | `""` | A name to substitute for the full names of resources. |
| globalOptions | string | `""` | Inject global options in the Caddyfile. |
| healthCheck | object | `{"enabled":true,"liveness":{"failureThreshold":3,"initialDelaySeconds":15,"periodSeconds":10,"timeoutSeconds":5},"readiness":{"failureThreshold":2,"initialDelaySeconds":5,"periodSeconds":5,"timeoutSeconds":3}}` | Transport-aware health checks exposed via the Caddy admin API. When enabled, readiness and liveness probes use /mercure/health/ready and /mercure/health/live on the admin port instead of /healthz on the HTTP port. |
| healthCheck.enabled | bool | `true` | Enable transport-aware health checks. |
| httpRoute | object | `{"annotations":{},"enabled":false,"hostnames":["mercure-example.local"],"parentRefs":[{"name":"gateway","sectionName":"http"}],"rules":[]}` | Expose the service via gateway-api HTTPRoute Requires Gateway API resources and suitable controller installed within the cluster (see: https://gateway-api.sigs.k8s.io/guides/) |
| httpRoute.annotations | object | `{}` | HTTPRoute annotations. |
| httpRoute.enabled | bool | `false` | HTTPRoute enabled. |
| httpRoute.hostnames | list | `["mercure-example.local"]` | Hostnames matching HTTP header. |
| httpRoute.parentRefs | list | `[{"name":"gateway","sectionName":"http"}]` | Which Gateways this Route is attached to. |
| httpRoute.rules | list | See [values.yaml](values.yaml). | List of rules and filters applied. When empty, a default rule routing all traffic to the service is created with `timeouts.request: 0s` so long-lived SSE subscriptions aren't cut by the gateway (most controllers default to a finite timeout, e.g. Envoy: 15s). Mercure's own `write_timeout` (default 600s) drives subscriber rotation. |
| image.pullPolicy | string | `"IfNotPresent"` | [Image pull policy](https://kubernetes.io/docs/concepts/containers/images/#updating-images) for updating already existing images on a node. |
| image.repository | string | `"dunglas/mercure"` | Name of the image repository to pull the container image from. |
| image.tag | string | `""` | Overrides the image tag whose default is the chart appVersion. |
| imagePullSecrets | list | `[]` | Reference to one or more secrets to be used when [pulling images](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#create-a-pod-that-uses-your-secret) (from private registries). |
| ingress.annotations | object | `{}` | Annotations to be added to the ingress. |
| ingress.className | string | `""` | Ingress [class name](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-class). |
| ingress.enabled | bool | `false` | Enable [ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/). |
| ingress.hosts | list | See [values.yaml](values.yaml). | Ingress host configuration. |
| ingress.tls | list | See [values.yaml](values.yaml). | Ingress TLS configuration. |
| license | string | `""` | The license key for [the High Availability version](https://mercure.rocks/docs/hub/cluster) (not necessary is you use the FOSS version). |
| metrics.enabled | bool | `false` | Enable metrics. You must also add a `servers` block with a [`metrics` directive](https://caddyserver.com/docs/caddyfile/options#metrics) in the `globalOptions` value. servers {     metrics } |
| metrics.port | int | `2019` | Deprecated: The port to use for exposing the metrics (use adminPort instead). |
| metrics.serviceMonitor.enabled | bool | `false` | Whether to create a ServiceMonitor for Prometheus Operator. |
| metrics.serviceMonitor.honorLabels | bool | `false` | Specify honorLabels parameter to add the scrape endpoint |
| metrics.serviceMonitor.interval | string | `"15s"` | The interval to use for the ServiceMonitor to scrape the metrics. |
| metrics.serviceMonitor.metricRelabelings | list | `[]` | RelabelConfigs to apply to samples before ingestion (sample relabeling). |
| metrics.serviceMonitor.relabelings | list | `[]` | RelabelConfigs to apply to samples before scraping (target relabeling). |
| metrics.serviceMonitor.scrapeTimeout | string | `""` | Timeout after which the scrape is ended |
| metrics.serviceMonitor.selector | object | `{}` | Additional labels that can be used so ServiceMonitor will be discovered by Prometheus |
| nameOverride | string | `""` | A name in place of the chart name for `app:` labels. |
| networkPolicy | object | Disabled by default. | [NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/) for the hub pods. When enabled with no ingress/egress rules, all traffic to/from the hub pods is denied. Supply rules to allow what you need. |
| networkPolicy.egress | list | `[]` | Egress rules (allowed outbound traffic). Pass-through to NetworkPolicy `spec.egress`. Allow at least DNS (UDP/TCP 53 to kube-system) plus the transport backend port. |
| networkPolicy.enabled | bool | `false` | Enable the NetworkPolicy. |
| networkPolicy.ingress | list | `[]` | Ingress rules (allowed inbound traffic). Pass-through to NetworkPolicy `spec.ingress`. With `policyTypes: [Ingress]` and `ingress: []`, all inbound traffic is denied. |
| networkPolicy.policyTypes | list | `[]` | `policyTypes` for the NetworkPolicy. The chart always renders both `ingress` and `egress` (defaulting to empty lists), so Kubernetes infers `policyTypes: [Ingress, Egress]`. Override (e.g. to `[Ingress]`) to opt out of egress restrictions. |
| nodeSelector | object | `{}` | [Node selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector) configuration. |
| persistence | object | `{"accessMode":"ReadWriteOnce","enabled":false,"existingClaim":"","size":"1Gi","storageClass":""}` | Enable persistence using [Persistent Volume Claims](http://kubernetes.io/docs/user-guide/persistent-volumes/), only useful if you the BoltDB transport. |
| persistence.accessMode | string | `"ReadWriteOnce"` | A manually managed Persistent Volume and Claim. Requires `persistence.enabled: true` If defined, PVC must be created manually before volume will be bound. |
| persistence.existingClaim | string | `""` | If defined, PVC must be created manually before volume will be bound |
| persistence.storageClass | string | `""` | Mercure Data Persistent Volume Storage Class. If defined, `storageClassName: <storageClass>` If set to `"-"``, `storageClassName: ""``, which disables dynamic provisioning. If undefined (the default) or set to `null`, no `storageClassName` spec is set, choosing the default provisioner. |
| podAnnotations | object | `{}` | Annotations to be added to pods. |
| podLabels | object | `{}` | Extra labels to be added to pods. |
| podSecurityContext | object | `{}` | Pod [security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod). See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#security-context) for details. |
| progressDeadlineSeconds | int | `1800` | Deployment `spec.progressDeadlineSeconds`. A rolling update of a hub with live SSE subscribers can legitimately spend up to `terminationGracePeriodSeconds` on each pod while the old replica drains — well above the k8s default of 600s. Too-low values make `helm upgrade --wait` (and similar CI gates) fail with `ProgressDeadlineExceeded` even though the rollout is healthy. Default covers a 2-pod rolling update with the defaults above; for larger `replicaCount`, scale up accordingly (roughly `replicaCount × (terminationGracePeriodSeconds + minReadySeconds)` plus a safety margin). Only applied when `updateStrategy.type` is `RollingUpdate`. |
| publisherJwtAlg | string | `"HS256"` | The JWT algorithm to use for publishers. |
| publisherJwtKey | string | `""` | The JWT key to use for publishers, a random key will be generated if empty. |
| replicaCount | int | `1` | The number of replicas (pods) to launch, must be 1 unless you are using [the High Availability version](https://mercure.rocks/docs/hub/cluster). |
| resources | object | No requests or limits. | Container resource [requests and limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/). See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#resources) for details. |
| securityContext | object | `{}` | Container [security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container). See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#security-context-1) for details. To run rootless on ports 80/443, add `NET_BIND_SERVICE` to the container capabilities (see the example below). The image's binary also carries `cap_net_bind_service` as a file capability, but `allowPrivilegeEscalation: false` enables `no_new_privs`, which tells the kernel to ignore file capabilities on `exec`, so the explicit `add` is what makes the bind work under that hardening. The chart mounts `/config` and `/tmp` as writable volumes so `readOnlyRootFilesystem: true` works out of the box; with the default BoltDB transport, also enable persistence so `/data` is a writable PVC. |
| service.annotations | object | `{}` |  |
| service.nodePort | string | `nil` | Set this, to pin the external nodePort in case `service.type` is `NodePort`. |
| service.port | int | `80` | Service port. |
| service.targetPort | int | `80` | Service target port. |
| service.type | string | `"ClusterIP"` | Kubernetes [service type](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types). |
| serviceAccount.annotations | object | `{}` | Annotations to add to the service account. |
| serviceAccount.automount | bool | `true` | Automatically mount a ServiceAccount's API credentials? |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created. |
| serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template. |
| subscriberJwtAlg | string | `"HS256"` | The JWT algorithm to use for subscribers. |
| subscriberJwtKey | string | `""` | The JWT key to use for subscribers, a random key will be generated if empty. |
| terminationGracePeriodSeconds | int | `660` | Pod terminationGracePeriodSeconds. Must be >= `write_timeout` plus a margin so the hub's graceful SSE drain (subscribers close at their own write deadline) finishes before k8s SIGKILLs the pod. Default matches Mercure's DefaultWriteTimeout (600s) plus a 60s margin. Only applied when `updateStrategy.type` is `RollingUpdate` — with `Recreate` we intentionally leave k8s to use its default (30s) to minimize the downtime window between old-pod-gone and new-pod-ready. |
| tolerations | list | `[]` | [Tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) for node taints. See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#scheduling) for details. |
| transportUrl | string | `""` | Deprecated: The URL representation of the transport to use. |
| updateStrategy | object | `{"type":"RollingUpdate"}` | [Deployment strategy type](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#strategy). Useful to set it to 'Recreate' when using BoltDB transport with persistence. |

