# Mercure, Server-Sent Live Updates
*Protocol and Reference Implementation*

Mercure is a protocol allowing to push data updates to web browsers and other HTTP clients in a fast, reliable and battery-efficient way.
It is especially useful to publish real-time updates of resources served through web APIs, to reactive web and mobile apps.

In addition to the full specification, a reference, production-grade implementation of **a Mercure server** (the hub) is provided in this repository. It is written in Go (golang) and is a free software licensed under the AGPL license.
It also includes a library that can be used in any Go application to implement the Mercure protocol directly (without a hub).

Mercure in a few words:

* native browser support, no lib nor SDK required (built on top of [server-sent events](https://www.smashingmagazine.com/2018/02/sse-websockets-data-flow-http2/))
* compatible with all existing servers, even those who don't support persistent connections (serverless, PHP, FastCGI...)
* builtin connection re-establishment and state reconciliation
* [JWT](https://jwt.io/)-based authorization mechanism (securely dispatch an update to some selected subscribers)
* performant, leverages [HTTP/2 multiplexing](https://developers.google.com/web/fundamentals/performance/http2/#request_and_response_multiplexing)
* designed with [hypermedia in mind](https://en.wikipedia.org/wiki/HATEOAS), also supports [GraphQL](https://graphql.org/)
* auto-discoverable through [web linking](https://tools.ietf.org/html/rfc5988)
* message encryption support
* can work with old browsers (IE7+) using an `EventSource` polyfill
* [connection-less push](https://html.spec.whatwg.org/multipage/server-sent-events.html#eventsource-push) in controlled environments (e.g. browsers on mobile handsets tied to specific carriers)

The reference Hub implementation:

* Fast, written in Go
* Works everywhere: static binaries and Docker images available
* Automatic HTTP/2 and HTTPS (using Let's Encrypt) support
* Cloud Native, follows [the Twelve-Factor App](https://12factor.net) methodoloy
* Open source (AGPL)

Example implementation of a client in JavaScript:

```javascript
// The subscriber subscribes to updates for the https://example.com/foo topic
// and to any topic matching https://example.com/books/{name}
const params = new URLSearchParams([
    ['topic', 'https://example.com/foo'],
    ['topic', 'https://example.com/books/{name}'],
]);
const eventSource = new EventSource(`https://hub.example.com?${params}`);

// The callback will be called every time an update is published
eventSource.onmessage = function ({data}) {
    console.log(data);
};
```

To dispatch an update (in any JS environment, including Node with the fetch polyfill):

```javascript
// ...
```

Example Use Cases:

**Live Availability**

* a Progressive Web App retrieves the availability status of a product from a REST API and displays it: only one is still
  available
* 3 minutes later, the last product is bought by another customer
* the PWA's view instantly show that this product isn't available anymore

**Asynchronous Jobs**

* a Progressive Web App tell the server to compute a report, this task is costly and will some time to finish
* the server delegates the computation of the report on an asynchronous worker (using message queue), and close the connection with the PWA
* the worker sends the report to the PWA when it is computed

**Collaborative Editing**

* a webapp allows several users to edit the same document concurently
* changes made are immediately broadcasted to all connected users

**Mercure gets you covered!**

## Protocol Specification

The full protocol specification can be found in [`spec/mercure.md`](spec/mercure.md).
It is also available as an [IETF's Internet Draft](https://www.ietf.org/id-info/),
and is designed to be published as a RFC.

## The Hub Implementation

### Usage

#### Prebuilt Binary

Grab a binary from the release page and run:

    PUBLISHER_JWT_KEY=myPublisherKey SUBSCRIBER_JWT_KEY=mySubcriberKey ACME_HOSTS=example.com ./mercure 

The ACME_HOSTS environment variable allows to use Let's Encrypt to expose a valid SSL certificate.
If you omit this variable, the server will be exposed on an (unsecure) HTTP connection.

#### Docker

A Docker image is available on Docker Hub. The following command is enough to get a working server:

    docker run \
        -e PUBLISHER_JWT_KEY=myPublisherKey -e SUBSCRIBER_JWT_KEY=mySubcriberKey -e ACME_HOSTS=example.com \
        -p 80:80 -p 443:443 \
        dunglas/mercure

### Environment Variables

* `PUBLISHER_JWT_KEY`: must contain the secret key to valid publishers' JWT
* `SUBSCRIBER_JWT_KEY`: must contain the secret key to valid subscribers' JWT
* `CORS_ALLOWED_ORIGINS`: a comma separated list of hosts allowed CORS origins
* `ALLOW_ANONYMOUS`:  set to `1` to allow subscribers with no valid JWT to connect
* `DEBUG`: set to `1` to enable the debug mode (prints recovery stack traces)
* `DEMO`: set to `1` to enable the demo mode (automatically enabled when `DEBUG=1`)
* `DB_PATH`: the path of the [bbolt](https://github.com/etcd-io/bbolt) database (default to `updates.db` in the current directory)
* `ACME_HOSTS`: a comma separated list of host for which Let's Encrypt certificates must be issues
* `ACME_CERT_DIR`: the directory where to store Let's Encrypt certificates
* `CERT_FILE`: a cert file (to use a custom certificate)
* `CERT_KEY`: a cert key (to use a custom certificate)
* `LOG_FORMAT`: the log format, can be `JSON`, `FLUENTD` or `TEXT` (default)

If `ACME_HOSTS` or both `CERT_FILE` and `CERT_KEY` are provided, an HTTPS server supporting HTTP/2 connection will be started.
If not, an HTTP server will be started (**not secure**).

## Resources

* [JavaScript library to decrypt JWE using the WebCrypto API](https://github.com/square/js-jose)
* [`EventSource` polyfill for old browsers](https://github.com/Yaffle/EventSource)
* [`EventSource` implementation for Node](https://github.com/EventSource/eventsource)
* [`Server-sent events` client (and server) for Go](https://github.com/donovanhide/eventsource)
