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

| Version | Branch | Status                                                                                                              |
| ------- | ------ | ------------------------------------------------------------------------------------------------------------------- |
| 1.0.x   | `main` | Active development. `1.0.0-beta.x` prereleases are shipped from `main`; not recommended for production until 1.0.0. |
| 0.24.x  | `0.x`  | Security and critical bugfixes only, until `1.0.0` ships.                                                           |
| < 0.24  | —      | Unsupported. Upgrade to 0.24.x.                                                                                     |

Patches for the 0.x line land on the `0.x` branch and are released from
there (e.g., `0.24.2`). Fixes that also apply to v1 are cherry-picked to
`main`.
