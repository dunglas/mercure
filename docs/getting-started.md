# Getting Started

![Subscriptions Schema](../spec/subscriptions.png)

## Starting the Hub

The easiest way to get started is to [install the official Mercure.rocks
Hub](hub/install.md). When it's done, go directly to the next step. There are also other unofficial [libraries implementing Mercure](ecosystem/awesome.md#hubs-and-server-libraries). In the rest of this tutorial, we'll assume that the hub is running on `https://localhost` and that the `JWT_KEY` is `!ChangeThisMercureHubJWTSecretKey!`.

Please note that the hub is entirely optional when using the Mercure protocol. Your app can also implement the Mercure protocol directly.

## Subscribing

Subscribing to updates from a web browser or any other platform supporting [Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events) is straightforward:

```javascript
// The subscriber subscribes to updates for the https://example.com/users/dunglas topic
// and to any topic matching https://example.com/books/{id}
const url = new URL('https://localhost/.well-known/mercure');
url.searchParams.append('topic', 'https://example.com/books/{id}');
url.searchParams.append('topic', 'https://example.com/users/dunglas');
// The URL class is a convenient way to generate URLs such as https://localhost/.well-known/mercure?topic=https://example.com/books/{id}&topic=https://example.com/users/dunglas

const eventSource = new EventSource(url);

// The callback will be called every time an update is published
eventSource.onmessage = e => console.log(e); // do something with the payload
```

The `EventSource` class is available [in all modern web browsers](https://caniuse.com/eventsource). And for legacy browsers, [there are polyfills](ecosystem/awesome.md#useful-related-libraries).

## Closing Connection

It is important to close this connection between the client and the hub if it is no longer needed.
Opened connections have a continuous buffer that will drain your application resources.
This is especially true when using Single Page Applications (based on e.g. ReactJS): the connection is maintained even if the component that created it is unmounted.

To close the connection, call `eventSource.close()`.

## Sending Private Updates

Optionally, [the authorization mechanism](../spec/mercure.md#authorization) can be used to subscribe to private updates.

![Authorization Schema](../spec/authorization.png)

## Discovering the Mercure Hub

Also optionally, the hub URL can be automatically discovered:

![Discovery Schema](../spec/discovery.png)

Here is a snippet to extract the URL of the hub from the `Link` HTTP header.

```javascript
fetch('https://example.com/books/1') // Has this header `Link: <https://localhost/.well-known/mercure>; rel="mercure"`
    .then(response => {
        // Extract the hub URL from the Link header
        const hubUrl = response.headers.get('Link').match(/<([^>]+)>;\s+rel=(?:mercure|"[^"]*mercure[^"]*")/)[1];
        // Subscribe to updates using the first snippet, do something with response's body...
    });
```

## Publishing

To dispatch an update, the publisher (an application server, a web browser...) needs to send a `POST` HTTP request to the hub:

```http
POST example.com HTTP/1.1
Bearer eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwie3NjaGVtZX06Ly97K2hvc3R9L2RlbW8vYm9va3Mve2lkfS5qc29ubGQiLCIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey90b3BpY317L3N1YnNjcmliZXJ9Il0sInBheWxvYWQiOnsidXNlciI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdXNlcnMvZHVuZ2xhcyIsInJlbW90ZUFkZHIiOiIxMjcuMC4wLjEifX19.KKPIikwUzRuB3DTpVw6ajzwSChwFw5omBMmMcWKiDcM

topic=https://example.com/books/1&data={"foo": "updated value"}
```

Example using [curl](https://curl.haxx.se/):

```bash
curl -d 'topic=https://example.com/books/1' -d 'data={"foo": "updated value"}' -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwie3NjaGVtZX06Ly97K2hvc3R9L2RlbW8vYm9va3Mve2lkfS5qc29ubGQiLCIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey90b3BpY317L3N1YnNjcmliZXJ9Il0sInBheWxvYWQiOnsidXNlciI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdXNlcnMvZHVuZ2xhcyIsInJlbW90ZUFkZHIiOiIxMjcuMC4wLjEifX19.KKPIikwUzRuB3DTpVw6ajzwSChwFw5omBMmMcWKiDcM' -X POST https://localhost/.well-known/mercure
```

Example using [Node.js](https://nodejs.org/) / [Serverless](https://serverless.com/):

```javascript
// Handle a POST, PUT, PATCH or DELETE request or finish an async job...
// and notify the hub
const http = require('http');
const querystring = require('querystring');

const postData = querystring.stringify({
    'topic': 'https://example.com/books/1',
    'data': JSON.stringify({ foo: 'updated value' }),
});

const req = http.request({
    hostname: 'localhost',
    port: '3000',
    path: '/.well-known/mercure',
    method: 'POST',
    headers: {
        Authorization: 'Bearer eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwie3NjaGVtZX06Ly97K2hvc3R9L2RlbW8vYm9va3Mve2lkfS5qc29ubGQiLCIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey90b3BpY317L3N1YnNjcmliZXJ9Il0sInBheWxvYWQiOnsidXNlciI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdXNlcnMvZHVuZ2xhcyIsInJlbW90ZUFkZHIiOiIxMjcuMC4wLjEifX19.KKPIikwUzRuB3DTpVw6ajzwSChwFw5omBMmMcWKiDcM',
        // the JWT must have a mercure.publish key containing an array of topic selectors (can contain "*" for all topics, and be empty for public updates)
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

The JWT must contain a `publish` property containing an array of topic selectors.
This array can be empty to allow publishing anonymous updates only.
The topic selector `*` can be used to allow publishing private updates for all topics. To create and read JWTs try [jwt.io](https://jwt.io) ([demo token](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwie3NjaGVtZX06Ly97K2hvc3R9L2RlbW8vYm9va3Mve2lkfS5qc29ubGQiLCIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey90b3BpY317L3N1YnNjcmliZXJ9Il0sInBheWxvYWQiOnsidXNlciI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdXNlcnMvZHVuZ2xhcyIsInJlbW90ZUFkZHIiOiIxMjcuMC4wLjEifX19.KKPIikwUzRuB3DTpVw6ajzwSChwFw5omBMmMcWKiDcM), key: `!ChangeThisMercureHubJWTSecretKey!`).

## Active Subscriptions

Mercure dispatches events every time a new subscription is created or terminated. It also exposes a web API to retrieve the list of active subscriptions.

[Learn more about subscriptions](../spec/mercure.md#active-subscriptions).

## Going Further

* [Read the full specification](../spec/mercure.md)
* [Install the hub](hub/install.md)
* [Checkout the examples](ecosystem/awesome.md#examples)
