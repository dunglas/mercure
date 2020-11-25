<h1 align="center"><a href="https://mercure.rocks"><img src="public/mercure.svg" alt="Mercure: Real-time Made Easy" title="Live Updates Made Easy"></a></h1>

*Protocol and Reference Implementation*

Mercure is a protocol allowing to push data updates to web browsers and other HTTP clients in a convenient, fast, reliable and battery-efficient way.
It is especially useful to publish async and real-time updates of resources served through web APIs, to reactive web and mobile apps.

[![Awesome](https://awesome.re/badge.svg)](docs/ecosystem/awesome.md)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/dunglas/mercure)](https://pkg.go.dev/github.com/dunglas/mercure)
![CI](https://github.com/dunglas/mercure/workflows/CI/badge.svg)
[![Coverage Status](https://coveralls.io/repos/github/dunglas/mercure/badge.svg?branch=master)](https://coveralls.io/github/dunglas/mercure?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/dunglas/mercure)](https://goreportcard.com/report/github.com/dunglas/mercure)

![Subscriptions Schema](spec/subscriptions.png)

* [Getting started](https://mercure.rocks/docs/getting-started)
* [Full documentation](https://mercure.rocks/docs)
* [Demo](https://demo.mercure.rocks/)

The protocol has been published as [an Internet Draft](https://datatracker.ietf.org/doc/draft-dunglas-mercure/) that [is maintained in this repository](https://mercure.rocks/spec).

A reference, production-grade, implementation of [**a Mercure hub**](https://mercure.rocks/docs/hub/install) (the server) is also available in this repository.
It's a free software (AGPL) written in Go. It is provided along with a library that can be used in any Go application to implement the Mercure protocol directly (without a hub) and an official Docker image.

In addition, a managed and high-scalability version of the Mercure.rocks hub is [available on Mercure.rocks](https://mercure.rocks/pricing).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License and Copyright 

See https://mercure.rocks/docs/hub/license.

## Credits

Created by [Kévin Dunglas](https://dunglas.fr). Graphic design by [Laury Sorriaux](https://github.com/ginifizz).
Sponsored by [Les-Tilleuls.coop](https://les-tilleuls.coop).
