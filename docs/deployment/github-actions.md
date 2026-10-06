---
title: "Run a Mercure hub service container in GitHub Actions"
description: "Run a Mercure.rocks Hub as a GitHub Actions service container for integration tests, with healthcheck and JWT publishing."
---

# Run Mercure in GitHub Actions

Run a Mercure hub for integration tests with a [service container](https://docs.github.com/en/actions/using-containerized-services/about-service-containers).

```yaml
name: CI

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    permissions:
      contents: read

    services:
      mercure:
        image: dunglas/mercure
        env:
          SERVER_NAME: ":1337"
          MERCURE_PUBLISHER_JWT_KEY: "ci-only-publisher-key-never-reuse-it"
          MERCURE_SUBSCRIBER_JWT_KEY: "ci-only-subscriber-key-never-reuse-it"
          MERCURE_EXTRA_DIRECTIVES: |
            anonymous
            cors_origins *
        options: >-
          --health-cmd "wget -q -O /dev/null http://localhost:2019/mercure/health/ready"
          --health-interval 5s
          --health-timeout 3s
          --health-retries 10
        ports:
          - 1337:1337

    steps:
      - uses: actions/checkout@v7

      - name: Run tests
        env:
          MERCURE_URL: http://localhost:1337/.well-known/mercure
        run: ./run-tests.sh
```

The hub is reachable at `http://localhost:1337/.well-known/mercure` from job steps. This HTTP setup is for an isolated CI environment. The default issuer is `https://localhost`; test tokens need the audience `http://localhost:1337/.well-known/mercure`.

## Healthcheck before tests start

The service's `options` configure a Docker health check that sends `GET` to the transport readiness endpoint. GitHub Actions waits for the service to become healthy before running job steps. See [GitHub service containers](https://docs.github.com/en/actions/using-containerized-services/about-service-containers).

Do not probe the subscription endpoint and interpret an error response as readiness.

## Sending updates from a workflow

To publish from inside a workflow (notify a Mercure-driven status page when a deploy finishes, post a Slack-style notification through your own hub):

```yaml
- name: Notify
  run: |
    curl --fail-with-body -X POST "$MERCURE_URL" \
      -H "Authorization: Bearer $MERCURE_JWT" \
      --data-urlencode "topic=https://example.com/deploys/${{ github.repository }}" \
      --data-urlencode "data={\"status\":\"deployed\",\"sha\":\"${{ github.sha }}\"}"
  env:
    MERCURE_URL: https://hub.example.com/.well-known/mercure
    MERCURE_JWT: ${{ secrets.MERCURE_PUBLISHER_JWT }}
```

Use a publisher token scoped to the notification topic. Store it as a repository secret and renew it before expiry. For automated rotation, issue short-lived tokens through your authorization service.

## Existing Mercure GitHub Actions

- [`Ilshidur/action-mercure`](https://github.com/Ilshidur/action-mercure) wraps the publish call into a reusable Action.

## Tips for Mercure in GitHub Actions workflows

- **Run health checks inside the service container.** The example queries the loopback admin endpoint without exposing it to the runner.
- **Use a fixed port.** `1337` is conventional; pick one that won't collide with other services in your matrix.
- **Don't share JWTs across forks.** Repository secrets aren't exposed to PRs from forks; keep that in mind for any workflow that publishes externally.
