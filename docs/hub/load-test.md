# Load Test

According to a benchmark made by Glory4Gamers, the open source version of the Mercure.rocks hub is able to 40k concurrent connections on a single EC2 t3.micro instance.
It's even possible to handle way more connections by using [the HA version of the Mercure.rocks Hub](cluster.md).

To test your own infrastructure, we provide a [Gatling](https://gatling.io)-based load test. It allows to test any implementation of the protocol, including the open source Hub.

## Running the Load Test

1. Download [Gatling version 3](https://gatling.io/open-source/)
2. Clone the Mercure repository: `git clone https://github.com/dunglas/mercure`
3. Run `path/to/gatling/bin/gatling.sh --simulations-folder mercure/gatling/`

## Configuration

Available environment variables (all are optional):

* `HUB_URL`: the URL of the hub to test
* `JWT`: the JWT to use for authenticating publishers
* `SUBSCRIBER_JWT`: the JWT to use for authenticating subscribers, fallbacks to `JWT` not set and `PRIVATE_UPDATES` set
* `INITIAL_SUBSCRIBERS`: the number of concurrent subscribers initially connected
* `SUBSCRIBERS_RATE_FROM`: minimum rate (per second) of additional subscribers to connect
* `SUBSCRIBERS_RATE_TO`: maximum rate (per second) of additional subscribers to connect
* `PUBLISHERS_RATE_FROM`: minimum rate (per second) of publications
* `PUBLISHERS_RATE_TO`: maximum rate (per second) of publications
* `INJECTION_DURATION`: duration of the publishers injection
* `CONNECTION_DURATION`: duration of subscribers' connection
* `RANDOM_CONNECTION_DURATION`: to randomize the connection duration (will longs `CONNECTION_DURATION` at max)
* `PRIVATE_UPDATES`: to send private updates with random topics instead of public updates always with the same topic

## See Also

* [Conformance tests](../ecosystem/conformance-tests.md)
