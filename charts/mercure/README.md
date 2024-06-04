<!-- markdownlint-disable -->
# Mercure Chart for Kubernetes

![Version: 0.16.2](https://img.shields.io/badge/Version-0.16.2-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.16.2](https://img.shields.io/badge/AppVersion-v0.16.2-informational?style=flat-square)

A Helm chart to install a Mercure Hub in a Kubernetes cluster. Mercure is a protocol to push data updates to web browsers and other HTTP clients in a convenient, fast, reliable and battery-efficient way.

[Learn more about Mercure.](https://mercure.rocks)

## Installing the Chart

To install the chart with the release name `my-release`, run the following commands:

    helm repo add mercure https://charts.mercure.rocks
    helm install my-release mercure/mercure

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | [Affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) configuration. See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#scheduling) for details. |
| autoscaling | object | Disabled by default. | Autoscaling must not be enabled unless you are using [the High Availability version](https://mercure.rocks/docs/hub/cluster) (see [values.yaml](values.yaml) for details). |
| caddyExtraConfig | string | `""` | Inject snippet or named-routes options in the Caddyfile |
| caddyExtraDirectives | string | `""` | Inject extra Caddy directives in the Caddyfile. |
| dev | bool | `false` | Enable the development mode, including the debug UI and the demo. |
| extraDirectives | string | `""` | Inject extra Mercure directives in the Caddyfile. |
| fullnameOverride | string | `""` | A name to substitute for the full names of resources. |
| globalOptions | string | `""` | Inject global options in the Caddyfile. |
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
| nameOverride | string | `""` | A name in place of the chart name for `app:` labels. |
| nodeSelector | object | `{}` | [Node selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector) configuration. |
| persistence | object | `{"accessMode":"ReadWriteOnce","enabled":false,"existingClaim":"","size":"1Gi","storageClass":""}` | Enable persistence using [Persistent Volume Claims](http://kubernetes.io/docs/user-guide/persistent-volumes/), only useful if you the BoltDB transport. |
| persistence.accessMode | string | `"ReadWriteOnce"` | A manually managed Persistent Volume and Claim. Requires `persistence.enabled: true` If defined, PVC must be created manually before volume will be bound. |
| persistence.existingClaim | string | `""` | If defined, PVC must be created manually before volume will be bound |
| persistence.storageClass | string | `""` | Mercure Data Persistent Volume Storage Class. If defined, `storageClassName: <storageClass>` If set to `"-"``, `storageClassName: ""``, which disables dynamic provisioning. If undefined (the default) or set to `null`, no `storageClassName` spec is set, choosing the default provisioner. |
| podAnnotations | object | `{}` | Annotations to be added to pods. |
| podSecurityContext | object | `{}` | Pod [security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod). See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#security-context) for details. |
| publisherJwtAlg | string | `"HS256"` | The JWT algorithm to use for publishers. |
| publisherJwtKey | string | `""` | The JWT key to use for publishers, a random key will be generated if empty. |
| replicaCount | int | `1` | The number of replicas (pods) to launch, must be 1 unless you are using [the High Availability version](https://mercure.rocks/docs/hub/cluster). |
| resources | object | No requests or limits. | Container resource [requests and limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/). See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#resources) for details. |
| securityContext | object | `{}` | Container [security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container). See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#security-context-1) for details. |
| service.annotations | object | `{}` |  |
| service.port | int | `80` | Service port. |
| service.targetPort | int | `80` | Service target port. |
| service.type | string | `"ClusterIP"` | Kubernetes [service type](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types). |
| serviceAccount.annotations | object | `{}` | Annotations to add to the service account. |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created. |
| serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template. |
| subscriberJwtAlg | string | `"HS256"` | The JWT algorithm to use for subscribers. |
| subscriberJwtKey | string | `""` | The JWT key to use for subscribers, a random key will be generated if empty. |
| tolerations | list | `[]` | [Tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) for node taints. See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#scheduling) for details. |
| transportUrl | string | `"bolt:///data/mercure.db"` | The URL representation of the transport to use. |

