# Using a Mercure Service in GitHub Actions

Adding a Mercure hub in your Continuous Integration system powered by GitHub Actions is straightforward:
create a [service container](https://docs.github.com/en/free-pro-team@latest/actions/guides/about-service-containers) and map its port on the host:

```yaml
name: Create a Mercure service

on: push

jobs:
    my-job-using-mercure:
        runs-on: ubuntu-latest

        services:
            mercure:
                image: dunglas/mercure
                env:
                    SERVER_NAME: :1337
                    MERCURE_PUBLISHER_JWT_KEY: '!ChangeThisMercureHubJWTSecretKey!'
                    MERCURE_SUBSCRIBER_JWT_KEY: '!ChangeThisMercureHubJWTSecretKey!'
                    MERCURE_EXTRA_DIRECTIVES: |
                        # Custom directives, see https://mercure.rocks/docs/hub/config
                        anonymous
                        cors_origins *
                ports:
                    - 1337:1337
        steps:
            # ...
```

A Mercure hub is available at the address `http://localhost:1337/.well-known/mercure`.
