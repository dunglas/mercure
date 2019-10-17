# Load Test

According to a benchmark made by Glory4Gamers, the open source version of the Mercure hub is able to 40k concurrent connections on a single EC2 t3.micro instance.

To test your own infrastructure, we provide a [Gatling](https://gatling.io)-based load test. It allows to test any implementation of the protocol, including the open source Hub.

## Running the Load Test

1. Download [Gatling version 3](https://gatling.io/open-source/)
2. Clone the Mercure repository and go into it: `git clone https://github.com/dunglas/mercure && cd mercure`
3. Run `path/to/gatling/bin/gatling.sh` --simulations-folder .

## Configuration

Available environment variables (all are optional):

* `HUB_URL`: the URL of the hub to test
* `JWT`: the JWT to use for authenticating the publisher
* `SUBSCRIBERS`: the number of concurrent subscribers
* `PUBLISHERS`: the number of concurrent publishers
