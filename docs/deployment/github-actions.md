---
title: "Run a Mercure Hub Service Container in GitHub Actions"
description: "Run a Mercure.rocks Hub as a GitHub Actions service container for integration tests, with healthcheck and JWT publishing."
---

# GitHub Actions

Need a Mercure hub for integration tests? Use a [service container](https://docs.github.com/en/actions/using-containerized-services/about-service-containers).

```yaml
# .github/workflows/ci.yml
name: CI

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      mercure:
        image: dunglas/mercure
        env:
          SERVER_NAME: ":1337"
          MERCURE_PUBLISHER_JWT_KEY: "!ChangeThisMercureHubJWTSecretKey!"
          MERCURE_SUBSCRIBER_JWT_KEY: "!ChangeThisMercureHubJWTSecretKey!"
          MERCURE_EXTRA_DIRECTIVES: |
            anonymous
            cors_origins *
        ports:
          - 1337:1337

    steps:
      - uses: actions/checkout@v4

      - name: Run tests
        env:
          MERCURE_URL: http://localhost:1337/.well-known/mercure
        run: ./run-tests.sh
```

The hub is reachable at `http://localhost:1337/.well-known/mercure` from any step. `anonymous` and `cors_origins *` are convenient for tests; don't copy them to production.

## Healthcheck before tests start

Service containers start in parallel with the job. If your test relies on the hub being responsive, wait for it:

```yaml
# Healthcheck before tests start
steps:
  - name: Wait for Mercure
    run: |
      for i in $(seq 1 30); do
        if curl -sf http://localhost:1337/.well-known/mercure -o /dev/null -w '%{http_code}' | grep -q 405; then
          echo "Hub is up"
          exit 0
        fi
        sleep 1
      done
      echo "Hub failed to start"
      exit 1
```

The hub returns `405 Method Not Allowed` on plain `GET /.well-known/mercure` (no `match=` query parameter). That's the simplest "the hub is alive" check.

## Sending updates from a workflow

To publish from inside a workflow (notify a Mercure-driven status page when a deploy finishes, post a Slack-style notification through your own hub):

```yaml
# Sending updates from a workflow
- name: Notify
  run: |
    curl -X POST "$MERCURE_URL" \
      -H "Authorization: Bearer $MERCURE_JWT" \
      -d "topic=https://example.com/deploys/${{ github.repository }}" \
      -d "data={\"status\":\"deployed\",\"sha\":\"${{ github.sha }}\"}"
  env:
    MERCURE_URL: https://hub.example.com/.well-known/mercure
    MERCURE_JWT: ${{ secrets.MERCURE_PUBLISHER_JWT }}
```

Mint the JWT once with a long-lived `exp` and store it as a repository secret. Rotate it when the underlying signing key rotates.

## Existing Mercure GitHub Actions

- [`Ilshidur/action-mercure`](https://github.com/Ilshidur/action-mercure) wraps the publish call into a reusable Action.

## Tips for Mercure in GitHub Actions Workflows

- **Service containers don't expose Caddy's admin port.** The `2019/mercure/health/ready` endpoint isn't reachable from the job runner. Use the `405` check above for readiness.
- **Use a fixed port.** `1337` is conventional; pick one that won't collide with other services in your matrix.
- **Don't share JWTs across forks.** Repository secrets aren't exposed to PRs from forks; keep that in mind for any workflow that publishes externally.
