<h1 align="center"><img src="public/mercure.svg" alt="Mercure: Live Updates Made Easy" title="Live Updates Made Easy"></h1>

*Protocol and Reference Implementation*

Mercure is a protocol allowing to push data updates to web browsers and other HTTP clients in a convenient, fast, reliable and battery-efficient way.
It is especially useful to publish real-time updates of resources served through web APIs, to reactive web and mobile apps.

[![GoDoc](https://godoc.org/github.com/dunglas/mercure?status.svg)](https://godoc.org/github.com/dunglas/mercure/hub)
[![Build Status](https://travis-ci.com/dunglas/mercure.svg?branch=master)](https://travis-ci.com/dunglas/mercure)
[![Coverage Status](https://coveralls.io/repos/github/dunglas/mercure/badge.svg?branch=master)](https://coveralls.io/github/dunglas/mercure?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/dunglas/mercure)](https://goreportcard.com/report/github.com/dunglas/mercure)

![Subscriptions Schema](spec/subscriptions.png)

The protocol has been published as [an Internet Draft](https://datatracker.ietf.org/doc/draft-dunglas-mercure/) that [is maintained in this repository](spec/mercure.md).

A reference, production-grade, implementation of **a Mercure hub** (the server) is also available here.
It's a free software (AGPL) written in Go. It is provided along with a library that can be used in any Go application to implement the Mercure protocol directly (without a hub) and an official Docker image.

[Try the demo!](https://demo.mercure.rocks/)

In addition, a managed and high-scalability version of Mercure is [available in private beta](mailto:dunglas+mercure@gmail.com?subject=I%27m%20interested%20in%20Mercure%27s%20private%20beta).

## Mercure in a Few Words

* native browser support, no lib nor SDK required (built on top of HTTP and [server-sent events](https://www.smashingmagazine.com/2018/02/sse-websockets-data-flow-http2/))
* compatible with all existing servers, even those who don't support persistent connections (serverless architecture, PHP, FastCGI...)
* built-in connection re-establishment and state reconciliation
* [JWT](https://jwt.io/)-based authorization mechanism (securely dispatch an update to some selected subscribers)
* performant, leverages [HTTP/2 multiplexing](https://developers.google.com/web/fundamentals/performance/http2/#request_and_response_multiplexing)
* designed with [hypermedia in mind](https://en.wikipedia.org/wiki/HATEOAS), also supports [GraphQL](https://graphql.org/)
* auto-discoverable through [web linking](https://tools.ietf.org/html/rfc5988)
* message encryption support
* can work with old browsers (IE7+) using an `EventSource` polyfill
* [connection-less push](https://html.spec.whatwg.org/multipage/server-sent-events.html#eventsource-push) in controlled environments (e.g. browsers on mobile handsets tied to specific carriers)

The reference hub implementation:

* Fast, written in Go
* Works everywhere: static binaries and Docker images available
* Automatic HTTP/2 and HTTPS (using Let's Encrypt) support
* Cloud Native, follows [the Twelve-Factor App](https://12factor.net) methodology
* Open source (AGPL)

# Examples

Example implementation of a client (the subscriber), in JavaScript:

```javascript
// The subscriber subscribes to updates for the https://example.com/foo topic
// and to any topic matching https://example.com/books/{name}
const url = new URL('https://example.com/hub');
url.searchParams.append('topic', 'https://example.com/books/{id}');
url.searchParams.append('topic', 'https://example.com/users/dunglas');

const eventSource = new EventSource(url);

// The callback will be called every time an update is published
eventSource.onmessage = e => console.log(e); // do something with the payload
```

Optionaly, [the authorization mechanism](https://github.com/dunglas/mercure/blob/master/spec/mercure.md#authorization) can be used to subscribe to private updates.
Also optionaly, the hub URL can be automatically discovered:

```javascript
fetch('https://example.com/books/1') // Has this header `Link: <https://example.com/hub>; rel="mercure"`
    .then(response => {
        // Extract the hub URL from the Link header
        const hubUrl = response.headers.get('Link').match(/<([^>]+)>;\s+rel=(?:mercure|"[^"]*mercure[^"]*")/)[1];
        // Subscribe to updates using the first snippet, do something with response's body...
    });
```

![Discovery Schema](spec/discovery.png)

To dispatch an update, the publisher (an application server, a web browser...) just need to send a `POST` HTTP request to the hub.
Example using [Node.js](https://nodejs.org/) / [Serverless](https://serverless.com/):

```javascript
// Handle a POST, PUT, PATCH or DELETE request or finish an async job...
// and notify the hub
const https = require('https');
const querystring = require('querystring');

const postData = querystring.stringify({
    'topic': 'https://example.com/books/1',
    'data': JSON.stringify({ foo: 'updated value' }),
});

const req = https.request({
    hostname: 'example.com',
    port: '443',
    path: '/hub',
    method: 'POST',
    headers: {
        Authorization: 'Bearer <valid-jwt-token>',
        // the JWT must have a mercure.pulish key containing an array of targets (can be empty for public updates)
        // the JWT key must be shared between the hub and the server
        'Content-Type': 'application/x-www-form-urlencoded',
        'Content-Length': Buffer.byteLength(postData),
    }
}, /* optional response handler */);
req.write(postData);
req.end();

// You'll probably prefer use the request library or the node-fetch polyfill in real projects,
// but any HTTP client, written in any language, will be just fine.
```

The JWT must contain a `publish` property containing an array of targets. This array can be empty to allow publishing anonymous updates only. [Example publisher JWT](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6WyJmb28iLCJiYXIiXSwicHVibGlzaCI6WyJmb28iXX19.LRLvirgONK13JgacQ_VbcjySbVhkSmHy3IznH3tA9PM) (demo key: `!UnsecureChangeMe!`).

Examples in other languages are available in [the `examples/` directory](examples/).

## Use Cases

### Live Availability

* a webapp retrieves the availability status of a product from a REST API and displays it: only one is still available
* 3 minutes later, the last product is bought by another customer
* the webapp's view instantly show that this product isn't available anymore

### Asynchronous Jobs

* a webapp tells the server to compute a report, this task is costly and will take some time to finish
* the server delegates the computation of the report on an asynchronous worker (using message queue), and close the connection with the webapp
* the worker sends the report to the webapp when it is computed

### Collaborative Editing

* a webapp allows several users to edit the same document concurently
* changes made are immediately broadcasted to all connected users

**Mercure gets you covered!**

## Protocol Specification

The full protocol specification can be found in [`spec/mercure.md`](spec/mercure.md).
It is also available as an [IETF's Internet Draft](https://www.ietf.org/id-info/),
and is designed to be published as a RFC.

## Hub Implementation

### Managed Version

A managed, high-scalability version of Mercure is available in private beta.
[Drop us a mail](mailto:dunglas+mercure@gmail.com?subject=I%27m%20interested%20in%20Mercure%27s%20private%20beta) for details and pricing.

### Usage

#### Prebuilt Binary

Grab a binary from the release page and run:

    JWT_KEY='myJWTKey' ADDR=':3000' DEMO=1 ALLOW_ANONYMOUS=1 PUBLISH_ALLOWED_ORIGINS='http://localhost:3000' ./mercure

The server is now available on `http://localhost:3000`, with the demo mode enabled. Because `ALLOW_ANONYMOUS` is set to `1`, anonymous subscribers are allowed.

To run it in production mode, and generate automatically a Let's Encrypt TLS certificate, just run the following command as root:

    JWT_KEY='myJWTKey' ACME_HOSTS='example.com' ./mercure

The value of the `ACME_HOSTS` environment variable must be updated to match your domain name(s).
A Let's Enctypt TLS certificate will be automatically generated.
If you omit this variable, the server will be exposed on an (unsecure) HTTP connection.

When the server is up and running, the following endpoints are available:

* `POST https://example.com/hub`: to publish updates
* `GET https://example.com/hub`: to subscribe to updates

See [the protocol](spec/mercure.md) for further informations.

To compile the development version and register the demo page, see [CONTRIBUTING.md](CONTRIBUTING.md#hub).

#### Docker Image

A Docker image is available on Docker Hub. The following command is enough to get a working server in demo mode:

    docker run \
        -e JWT_KEY='myJWTKey' -e DEMO=1 -e ALLOW_ANONYMOUS=1 -e PUBLISH_ALLOWED_ORIGINS='http://localhost' \
        -p 80:80 \
        dunglas/mercure

The server, in demo mode, is available on `http://localhost`. Anonymous subscribers are allowed.

In production, run:

    docker run \
        -e JWT_KEY='myJWTKey' -e ACME_HOSTS='example.com' \
        -p 80:80 -p 443:443 \
        dunglas/mercure

Be sure to update the value of `ACME_HOSTS` to match your domain name(s), a Let's Encrypt TLS certificate will be automatically generated.

### Environment Variables

* `ACME_CERT_DIR`: the directory where to store Let's Encrypt certificates
* `ACME_HOSTS`: a comma separated list of hosts for which Let's Encrypt certificates must be issued
* `ADDR`: the address to listen on (example: `127.0.0.1:3000`, default to `:http` or `:https` depending if HTTPS is enabled or not)
* `ALLOW_ANONYMOUS`:  set to `1` to allow subscribers with no valid JWT to connect
* `CERT_FILE`: a cert file (to use a custom certificate)
* `CERT_KEY`: a cert key (to use a custom certificate)
* `CORS_ALLOWED_ORIGINS`: a comma separated list of allowed CORS origins, can be `*` for all
* `DB_PATH`: the path of the [bbolt](https://github.com/etcd-io/bbolt) database (default to `updates.db` in the current directory)
* `DEBUG`: set to `1` to enable the debug mode (prints recovery stack traces)
* `DEMO`: set to `1` to enable the demo mode (automatically enabled when `DEBUG=1`)
* `JWT_KEY`: the JWT key to use for both publishers and subscribers
* `LOG_FORMAT`: the log format, can be `JSON`, `FLUENTD` or `TEXT` (default)
* `PUBLISH_ALLOWED_ORIGINS`: a comma separated list of origins allowed to publish (only applicable when using cookie-based auth)
* `PUBLISHER_JWT_KEY`: must contain the secret key to valid publishers' JWT, can be omited if `JWT_KEY` is set
* `SUBSCRIBER_JWT_KEY`: must contain the secret key to valid subscribers' JWT, can be omited if `JWT_KEY` is set

If `ACME_HOSTS` or both `CERT_FILE` and `CERT_KEY` are provided, an HTTPS server supporting HTTP/2 connection will be started.
If not, an HTTP server will be started (**not secure**).

### Troubleshooting

#### 401 Unauthorized

* Be sure to set a **secret key** (and not a JWT) in `JWT_KEY` (or in `SUBSCRIBER_JWT_KEY` and `PUBLISHER_JWT_KEY`)
* If the secret key contains special characters, be sure to escape them properly, especially if you set the environment variable in a shell, or in a YAML file (Kubernetes...)
* The publisher always needs a valid JWT, even if `ALLOW_ANONYMOUS` is set to `1`, this JWT **must** have a property named `publish` and containing an array of targets
* The subscriber needs a valid JWT only if `ALLOW_ANONYMOUS` is set to `0` (default), or to subscribe to private updates, in this case the JWT **must** have a property named `subscribe` and containing an array of targets

For both the `publish` and `subscribe` properties, the array can be empty to publish only public updates, or set it to `["*"]` to allow accessing to all targets.

## FAQ

### How to Use Mercure with GraphQL?

Because they are delivery agnostic, Mercure plays particulary well with [GraphQL's subscriptions](https://facebook.github.io/graphql/draft/#sec-Subscription).

In response to the subscription query, the GraphQL server may return a corresponding topic URL.
The client can then subscribe to the Mercure's event stream corresponding to this subscription by creating a new `EventSource` with an URL like `https://example.com/hub?topic=https://example.com/subscriptions/<subscription-id>` as parameter.

Updates for the given subscription can then be sent from the GraphQL server to the clients through the Mercure hub (in the `data` property of the server-sent event).

To unsubscribe, the client just calls `EventSource.close()`.

Mercure can easily be integrated with Apollo GraphQL by creating [a dedicated transport](https://github.com/apollographql/graphql-subscriptions).

### How to Use NGINX as an HTTP/2 Reverse Proxy in Front of Mercure?

NGINX is supported out of the box. Use the following proxy configuration:

```nginx
server {
    listen 80 ssl http2;
    listen [::]:80 ssl http2;

    ssl_certificate /path/to/ssl/cert.crt;
    ssl_certificate_key /path/to/ssl/cert.key;

    location / {
        proxy_pass http://url-of-your-mercure-hub;
        proxy_read_timeout 24h;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
    }
}
```

### What's the Difference Between Mercure and WebSocket?

[WebSocket](https://developer.mozilla.org/en-US/docs/Web/API/WebSockets_API) is a low level protocol, Mercure is a high level one.
Mercure provides convenient built-in features such as authorization, re-connection and state reconciliation ; while with WebSocket, you need to implement them yourself.
Also, unlike Mercure (which is built on top of HTTP and Server-Sent Events), WebSocket [is not designed to leverage HTTP/2](https://www.infoq.com/articles/websocket-and-http2-coexist).

HTTP/2 connections are multiplexed and bidirectional by default (it was not the case of HTTP/1).
When using Mercure over a h2 connection (recommended), your app can receive data through Server-Sent Events, and send data to the server with regular `POST` (or `PUT`/`PATCH`/`DELETE`) requests, with no overhead.

Basically, in most cases Mercure can be used as a modern and easier to use replacement for WebSocket.

### What's the Difference Between Mercure and WebSub?

[WebSub](https://www.w3.org/TR/websub/) is a server-to-server only protocol, while Mercure is also a server-to-client and client-to-client protocol.

Mercure has been heavily inspired by WebSub, and we tried to make the protocol as close as possible from the WebSub one.

Mercure uses Server-Sent Events to dispatch the updates, while WebSub use `POST` requests. Also, Mercure has an advanced authorization mechanism, and allows to subscribe to several topics with only one connection using templated URIs.

### What's the Difference Between Mercure and Web Push?

The [Push API](https://developer.mozilla.org/en-US/docs/Web/API/Push_API) is a simplex protocol [mainly designed](https://developers.google.com/web/fundamentals/push-notifications/) to send [notifications](https://developer.mozilla.org/en-US/docs/Web/API/Notifications_API) to devices currently not connected to the application.
In most implementations, the size of the payload to dispatch is very limited, and the messages are sent through the proprietary APIs and servers of the browsers' and operating systems' vendors.

On the other hand, Mercure is a duplex protocol designed to send live updates to devices currently connected to the web or mobile app. The payload is not limited, and the message goes directly from your servers to the clients.

In summary, use the Push API to send notifications to offline users (that will be available in Chrome, Android and iOS's notification centers), and use Mercure to receive and publish live updates when the user is using the app.

## Resources

* [PHP library to publish Mercure updates](https://github.com/symfony/mercure)
* [Official Mercure support for the Symfony framework](https://github.com/symfony/mercure-bundle)
* [`EventSource` polyfill for Edge/IE and old browsers](https://github.com/Yaffle/EventSource)
* [`EventSource` polyfill for React Native](https://github.com/jordanbyron/react-native-event-source)
* [JavaScript library to parse `Link` headers](https://github.com/thlorenz/parse-link-header)
* [JavaScript library to decrypt JWE using the WebCrypto API](https://github.com/square/js-jose)
* [`EventSource` implementation for Node](https://github.com/EventSource/eventsource)
* [Server-Sent Events client for Go](https://github.com/donovanhide/eventsource)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Credits

Created by [Kévin Dunglas](https://dunglas.fr). Schemas by [Laury Sorriaux](https://github.com/ginifizz).
Sponsored by [Les-Tilleuls.coop](https://les-tilleuls.coop).
