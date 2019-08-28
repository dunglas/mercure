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

### Managed Version

A managed, high-scalability version of Mercure is available in private beta.
[Drop us a mail](mailto:dunglas+mercure@gmail.com?subject=I%27m%20interested%20in%20Mercure%27s%20private%20beta) for details and pricing.

### Usage

#### Prebuilt Binary

Grab the binary corresponding to your operating system and architecture [from the release page](https://github.com/dunglas/mercure/releases), then run:

    JWT_KEY='myJWTKey' ADDR=':3000' DEMO=1 ALLOW_ANONYMOUS=1 CORS_ALLOWED_ORIGINS=* PUBLISH_ALLOWED_ORIGINS='http://localhost:3000' ./mercure

Note: Mac OS users must use the `Darwin` binary.

The server is now available on `http://localhost:3000`, with the demo mode enabled. Because `ALLOW_ANONYMOUS` is set to `1`, anonymous subscribers are allowed.

To run it in production mode, and generate automatically a Let's Encrypt TLS certificate, just run the following command as root:

    JWT_KEY='myJWTKey' ACME_HOSTS='example.com' ./mercure

The value of the `ACME_HOSTS` environment variable must be updated to match your domain name(s).
A Let's Encrypt TLS certificate will be automatically generated.
If you omit this variable, the server will be exposed using a not encrypted HTTP connection.

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

#### Kubernetes

To install Mercure in a [Kubernetes](https://kubernetes.io) cluster, use the official [Helm Chart](https://hub.helm.sh/charts/stable/mercure):

    helm install stable/mercure

### Environment Variables

* `ACME_CERT_DIR`: the directory where to store Let's Encrypt certificates
* `ACME_HOSTS`: a comma separated list of hosts for which Let's Encrypt certificates must be issued
* `ADDR`: the address to listen on (example: `127.0.0.1:3000`, default to `:http` or `:https` depending if HTTPS is enabled or not). Note that Let's Encrypt only supports the default port: to use Let's Encrypt, **do not set this variable**.
* `ALLOW_ANONYMOUS`:  set to `1` to allow subscribers with no valid JWT to connect
* `CERT_FILE`: a cert file (to use a custom certificate)
* `KEY_FILE`: a key file (to use a custom certificate)
* `COMPRESS`: set to `0` to disable HTTP compression support (default to enabled)
* `CORS_ALLOWED_ORIGINS`: a comma separated list of allowed CORS origins, can be `*` for all
* `DB_PATH`: the path of the [bbolt](https://github.com/etcd-io/bbolt) database (default to `updates.db` in the current directory)
* `DEBUG`: set to `1` to enable the debug mode (prints recovery stack traces)
* `DEMO`: set to `1` to enable the demo mode (automatically enabled when `DEBUG=1`)
* `HEARTBEAT_INTERVAL`: interval between heartbeats (useful with some proxies, and old browsers, default to `15s`, set to `0s` to disable)
* `HISTORY_SIZE`: size of the history (to retrieve lost messages using the `Last-Event-ID` header), set to `0` to never remove old events (default)
* `HISTORY_CLEANUP_FREQUENCY`: chances to trigger history cleanup when an update occurs, must be a number between `0` (never cleanup) and `1` (cleanup after every publication), default to `0.3`
* `JWT_KEY`: the JWT key to use for both publishers and subscribers
* `LOG_FORMAT`: the log format, can be `JSON`, `FLUENTD` or `TEXT` (default)
* `PUBLISH_ALLOWED_ORIGINS`: a comma separated list of origins allowed to publish (only applicable when using cookie-based auth)
* `PUBLISHER_JWT_KEY`: must contain the secret key to valid publishers' JWT, can be omited if `JWT_KEY` is set
* `READ_TIMEOUT`: maximum duration for reading the entire request, including the body, set to `0s` to disable (default), example: `2m`
* `SUBSCRIBER_JWT_KEY`: must contain the secret key to valid subscribers' JWT, can be omited if `JWT_KEY` is set
* `WRITE_TIMEOUT`: maximum duration before timing out writes of the response, set to `0s` to disable (default), example: `2m`
* `USE_FORWARDED_HEADERS`: set to `1` to use the `X-Forwarded-For`, and `X-Real-IP` for the remote (client) IP address, `X-Forwarded-Proto` or `X-Forwarded-Scheme` for the scheme (http or https), `X-Forwarded-Host` for the host and the RFC 7239 `Forwarded` header, which may include both client IPs and schemes. If this option is enabled, the reverse proxy must override or remove these headers or you will be at risk.

If `ACME_HOSTS` or both `CERT_FILE` and `KEY_FILE` are provided, an HTTPS server supporting HTTP/2 connection will be started.
If not, an HTTP server will be started (**not secure**).

### Troubleshooting

#### Windows

If you're having trouble getting the Hub running, you may have set an incorrect value for the environment variable `ADDR`. Use `ADDR=":3000"` (and not `ADDR="localhost:3000"`). Windows may ask you for allowing `mercure.exe` in your firewall.

#### 401 Unauthorized

* Check the logs written by the hub on `stderr`, they contain the exact reason why the token has been rejected
* Be sure to set a **secret key** (and not a JWT) in `JWT_KEY` (or in `SUBSCRIBER_JWT_KEY` and `PUBLISHER_JWT_KEY`)
* If the secret key contains special characters, be sure to escape them properly, especially if you set the environment variable in a shell, or in a YAML file (Kubernetes...)
* The publisher always needs a valid JWT, even if `ALLOW_ANONYMOUS` is set to `1`, this JWT **must** have a property named `publish` and containing an array of targets ([example](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOltdfX0.473isprbLWLjXmAaVZj6FIVkCdjn37SQpGjzWws-xa0))
* The subscriber needs a valid JWT only if `ALLOW_ANONYMOUS` is set to `0` (default), or to subscribe to private updates, in this case the JWT **must** have a property named `subscribe` and containing an array of targets ([example](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6W119fQ.s-6MlTvJ6vpsZ7ftmz3dvWpZznRxnxI0KlrZOHVo8Qc))

For both the `publish` and `subscribe` properties, the array can be empty to publish only public updates, or set it to `["*"]` to allow accessing to all targets.

#### Browser Issues

If subscribing to the `EventSource` in the browser doesn't work (the browser instantly disconnects from the stream or complains about CORS policy on receiving an event), check that you've set a proper value for `CORS_ALLOWED_ORIGINS` on running Mercure. It's fine to use `CORS_ALLOWED_ORIGINS=*` for your local development.

#### URI Template and Topics

Try [our URI template tester](https://uri-template-tester.mercure.rocks/) to ensure that the template matches the topic.

## FAQ

### How to Use Mercure with GraphQL?

Because they are delivery agnostic, Mercure plays particulary well with [GraphQL's subscriptions](https://facebook.github.io/graphql/draft/#sec-Subscription).

In response to the subscription query, the GraphQL server may return a corresponding topic URL.
The client can then subscribe to the Mercure's event stream corresponding to this subscription by creating a new `EventSource` with an URL like `https://example.com/hub?topic=https://example.com/subscriptions/<subscription-id>` as parameter.

Updates for the given subscription can then be sent from the GraphQL server to the clients through the Mercure hub (in the `data` property of the server-sent event).

To unsubscribe, the client just calls `EventSource.close()`.

Mercure can easily be integrated with Apollo GraphQL by creating [a dedicated transport](https://github.com/apollographql/graphql-subscriptions).

### How to Monitor the Hub Using Supervisor

Use the following file as a template to run the Mercure hub with [Supervisor](http://supervisord.org):

```ini
[program:mercure]
command=/path/to/mercure
process_name=%(program_name)s_%(process_num)s
numprocs=1
environment=JWT_KEY="myJWTKey"
directory=/tmp
autostart=true
autorestart=true
startsecs=5
startretries=10
user=www-data
redirect_stderr=false
stdout_capture_maxbytes=1MB
stderr_capture_maxbytes=1MB
stdout_logfile=/path/to/logs/mercure.out.log
stderr_logfile=/path/to/logs/mercure.error.log
```

Save this file to `/etc/supervisor/conf.d/mercure.conf`.
Run `supervisorctl reread` and `supervisorctl update` to activate and start the Mercure hub.

### How to Use NGINX as an HTTP/2 Reverse Proxy in Front of the Hub?

[NGINX](https://www.nginx.com) is supported out of the box. Use the following proxy configuration:

```nginx
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;

    ssl_certificate /path/to/ssl/cert.crt;
    ssl_certificate_key /path/to/ssl/cert.key;

    location / {
        proxy_pass http://url-of-your-mercure-hub;
        proxy_read_timeout 24h;
        proxy_http_version 1.1;
        proxy_set_header Connection "";

        ## Be sure to set USE_FORWARDED_HEADERS=1 to allow the hub to use those headers ##
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
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

Mercure uses Server-Sent Events to dispatch the updates, while WebSub use `POST` requests. Also, Mercure has an advanced authorization mechanism, and allows to subscribe to several topics with only one connection using URI templates.

### What's the Difference Between Mercure and Web Push?

The [Push API](https://developer.mozilla.org/en-US/docs/Web/API/Push_API) is a simplex protocol [mainly designed](https://developers.google.com/web/fundamentals/push-notifications/) to send [notifications](https://developer.mozilla.org/en-US/docs/Web/API/Notifications_API) to devices currently not connected to the application.
In most implementations, the size of the payload to dispatch is very limited, and the messages are sent through the proprietary APIs and servers of the browsers' and operating systems' vendors.

On the other hand, Mercure is a duplex protocol designed to send live updates to devices currently connected to the web or mobile app. The payload is not limited, and the message goes directly from your servers to the clients.

In summary, use the Push API to send notifications to offline users (that will be available in Chrome, Android and iOS's notification centers), and use Mercure to receive and publish live updates when the user is using the app.

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
