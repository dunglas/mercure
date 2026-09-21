# Security Policy

## Reporting a vulnerability

Report security issues through GitHub's private vulnerability reporting:
<https://github.com/dunglas/mercure/security/advisories/new>.

Do not open public issues or PRs for suspected vulnerabilities.

### Mercure Cloud

This policy covers the Hub in this repository. For the hosted service at
mercure.rocks, including its API, dashboard and the hubs we operate, report to
<contact+security@mercure.rocks> instead. That service publishes its own
scope, triage targets and safe-harbour terms, which authorise good-faith
research against it:
<https://mercure.rocks/legal/security#vulnerability-disclosure-policy>.

## Supported versions

| Version | Branch | Status                                  |
| ------- | ------ | --------------------------------------- |
| 1.0.x   | `main` | Supported. Security fixes and bugfixes. |
| < 1.0   | —      | Unsupported. Upgrade to 1.0.x.          |

Fixes land on `main` and are released from there (e.g., `1.0.1`). See the
[upgrade guide](docs/UPGRADE.md) to move off an unsupported version.
