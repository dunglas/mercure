# mercure-example-chat

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

A minimalist chat system, using Mercure and the Flask microframework to handle cookie authentication

## Values

| Key                                        | Type   | Default                                            | Description |
| ------------------------------------------ | ------ | -------------------------------------------------- | ----------- |
| affinity                                   | object | `{}`                                               |             |
| autoscaling.enabled                        | bool   | `false`                                            |             |
| autoscaling.maxReplicas                    | int    | `100`                                              |             |
| autoscaling.minReplicas                    | int    | `1`                                                |             |
| autoscaling.targetCPUUtilizationPercentage | int    | `80`                                               |             |
| cookieDomain                               | string | `".mercure.rocks"`                                 |             |
| fullnameOverride                           | string | `""`                                               |             |
| hubUrl                                     | string | `"https://demo.mercure.rocks/.well-known/mercure"` |             |
| image.pullPolicy                           | string | `"Always"`                                         |             |
| image.repository                           | string | `"dunglas/mercure-example-chat"`                   |             |
| image.tag                                  | string | `"latest"`                                         |             |
| imagePullSecrets                           | list   | `[]`                                               |             |
| ingress.annotations                        | object | `{}`                                               |             |
| ingress.className                          | string | `""`                                               |             |
| ingress.enabled                            | bool   | `false`                                            |             |
| ingress.hosts[0].host                      | string | `"chart-example.local"`                            |             |
| ingress.hosts[0].paths[0].path             | string | `"/"`                                              |             |
| ingress.hosts[0].paths[0].pathType         | string | `"ImplementationSpecific"`                         |             |
| ingress.tls                                | list   | `[]`                                               |             |
| jwtKey                                     | string | `"!ChangeThisMercureHubJWTSecretKey!"`             |             |
| livenessProbe.httpGet.path                 | string | `"/"`                                              |             |
| livenessProbe.httpGet.port                 | string | `"http"`                                           |             |
| messageUriTemplate                         | string | `"https://chat.example.com/messages/{id}"`         |             |
| nameOverride                               | string | `""`                                               |             |
| nodeSelector                               | object | `{}`                                               |             |
| podAnnotations                             | object | `{}`                                               |             |
| podLabels                                  | object | `{}`                                               |             |
| podSecurityContext                         | object | `{}`                                               |             |
| readinessProbe.httpGet.path                | string | `"/"`                                              |             |
| readinessProbe.httpGet.port                | string | `"http"`                                           |             |
| replicaCount                               | int    | `1`                                                |             |
| resources                                  | object | `{}`                                               |             |
| securityContext                            | object | `{}`                                               |             |
| service.port                               | int    | `80`                                               |             |
| service.type                               | string | `"ClusterIP"`                                      |             |
| serviceAccount.annotations                 | object | `{}`                                               |             |
| serviceAccount.automount                   | bool   | `true`                                             |             |
| serviceAccount.create                      | bool   | `true`                                             |             |
| serviceAccount.name                        | string | `""`                                               |             |
| tolerations                                | list   | `[]`                                               |             |
| volumeMounts                               | list   | `[]`                                               |             |
| volumes                                    | list   | `[]`                                               |             |

---

Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)
