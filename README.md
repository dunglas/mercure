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
* CORS support, CSRF protection mechanism
* Cloud Native, follows [the Twelve-Factor App](https://12factor.net) methodology
* Open source (AGPL)

## Examples

Example implementation of a client (the subscriber), in JavaScript:

```javascript
// The subscriber subscribes to updates for the https://example.com/users/dunglas topic
// and to any topic matching https://example.com/books/{id}
const url = new URL('https://example.com/hub');
url.searchParams.append('topic', 'https://example.com/books/{id}');
url.searchParams.append('topic', 'https://example.com/users/dunglas');

const eventSource = new EventSource(url);

// The callback will be called every time an update is published
eventSource.onmessage = e => console.log(e); // do something with the payload
```

Optionally, [the authorization mechanism](https://github.com/dunglas/mercure/blob/master/spec/mercure.md#authorization) can be used to subscribe to private updates.

![Authorization Schema](spec/authorization.png)

Also optionally, the hub URL can be automatically discovered:

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
        // the JWT must have a mercure.publish key containing an array of targets (can be empty for public updates)
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

The JWT must contain a `publish` property containing an array of targets. This array can be empty to allow publishing anonymous updates only. [Example publisher JWT](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6WyJmb28iLCJiYXIiXSwicHVibGlzaCI6WyJmb28iXX19.afLx2f2ut3YgNVFStCx95Zm_UND1mZJ69OenXaDuZL8) (demo key: `!ChangeMe!`).

### Other Examples

* [Python/Flask: chat application](examples/chat-python-flask) ([live demo](https://python-chat.mercure.rocks/))
* [Node.js: publishing](examples/publisher-node.js)
* [PHP: publishing](examples/publisher-php.php)
* [Ruby: publishing](examples/publisher-ruby.rb)

## Use Cases

Example of usage: the Mercure integration in [API Platform](https://api-platform.com/docs/client-generator):

![API Platform screencast](https://api-platform.com/client-generator-demo-d20c0f7f49b5655a3788d9c570c1c80a.gif)

### Live Availability

* a webapp retrieves the availability status of a product from a REST API and displays it: only one is still available
* 3 minutes later, the last product is bought by another customer
* the webapp's view instantly shows that this product isn't available anymore

### Asynchronous Jobs

* a webapp tells the server to compute a report, this task is costly and will take some time to complete
* the server delegates the computation of the report to an asynchronous worker (using message queue), and closes the connection with the webapp
* the worker sends the report to the webapp when it is computed

### Collaborative Editing

* a webapp allows several users to edit the same document concurrently
* changes made are immediately broadcasted to all connected users

**Mercure gets you covered!**

## Protocol Specification

The full protocol specification can be found in [`spec/mercure.md`](spec/mercure.md).
It is also available as an [IETF's Internet Draft](https://datatracker.ietf.org/doc/draft-dunglas-mercure/),
and is designed to be published as a RFC.

An [OpenAPI specification](https://www.openapis.org/) of the hub API is also available in [`spec/openapi.yaml`](spec/openapi.yaml).

## Hub Implementation

See [hub_implementation.md](https://github.com/dunglas/mercure/blob/master/hub_implementation.md).

## FAQ

See [faq.md](https://github.com/dunglas/mercure/blob/master/faq.md).

## Tools

* [PHP library to publish Mercure updates](https://github.com/symfony/mercure)
* [Official Mercure support for the Symfony framework](https://github.com/symfony/mercure-bundle)
* [Official Mercure support for the API Platform framework](https://api-platform.com/docs/core/mercure/)
* [Laravel Mercure Broadcaster](https://github.com/mvanduijker/laravel-mercure-broadcaster)
* [`EventSource` polyfill for Edge/IE and old browsers](https://github.com/Yaffle/EventSource)
* [`EventSource` polyfill for React Native](https://github.com/jordanbyron/react-native-event-source)
* [`EventSource` implementation for Node](https://github.com/EventSource/eventsource)
* [Server-Sent Events client for Go](https://github.com/donovanhide/eventsource)
* [JavaScript library to parse `Link` headers](https://github.com/thlorenz/parse-link-header)
* [JavaScript library to decrypt JWE using the WebCrypto API](https://github.com/square/js-jose)
* [Python library to publish and consume Mercure updates](https://github.com/vitorluis/python-mercure)

## Learning Resources

### English

* [Official Push and Real-Time Capabilities for Symfony and API Platform using Mercure (Symfony blog)](https://dunglas.fr/2019/03/official-push-and-real-time-capabilities-for-symfony-and-api-platform-mercure-protocol/)
* [Tech Workshop: Mercure by Kevin Dunglas at SensioLabs (SensioLabs)](https://blog.sensiolabs.com/2019/01/24/tech-workshop-mercure-kevin-dunglas-sensiolabs/)
* [Real-time messages with Mercure using Laravel](http://thedevopsguide.com/real-time-notifications-with-mercure/)

### French

* [Tutoriel vidéo : Notifications instantanées avec Mercure (Grafikart)](https://www.grafikart.fr/tutoriels/symfony-mercure-1151)
* [Mercure, un protocole pour pousser des mises à jour vers des navigateurs et app mobiles en temps réel (Les-Tilleuls.coop)](https://les-tilleuls.coop/fr/blog/article/mercure-un-protocole-pour-pousser-des-mises-a-jour-vers-des-navigateurs-et-app-mobiles-en-temps-reel)

## Load Testing

A [Gatling](https://gatling.io)-based load test is provided in this repository.
It allows to test any implementation of the protocol, including the reference implementation.

See [`LoadTest.scala`](LoadTest.scala) to learn how to use it.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Credits

Created by [Kévin Dunglas](https://dunglas.fr). Graphic design by [Laury Sorriaux](https://github.com/ginifizz).
Sponsored by [Les-Tilleuls.coop](https://les-tilleuls.coop).
