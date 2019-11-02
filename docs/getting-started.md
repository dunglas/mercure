# Getting Started

![Subscriptions Schema](../spec/subscriptions.png)

## Starting the Hub

The easiest way to get started is to [install the official Mercure Hub](hub/install.md). When it's done, go directly to the next step.
Also, keep in mind that the hub is entirely optional. By the specification, your app can implement the Mercure protocol directly (take a look to the [libraries implementing Mercure](ecosystem/awesome.md#hubs-and-server-libraries)).

In the rest of this tutorial, we'll assume that the hub is running on `http://localhost:3000` and that the `JWT_KEY` is `!ChangeMe!`.

## Subscribing

Subscribing to updates from a web browser or any other platform supporting [Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events) is straightforward:

```javascript
// The subscriber subscribes to updates for the https://example.com/users/dunglas topic
// and to any topic matching https://example.com/books/{id}
const url = new URL('https://example.com/.well-known/mercure');
url.searchParams.append('topic', 'https://example.com/books/{id}');
url.searchParams.append('topic', 'https://example.com/users/dunglas');
// The URL class is a convenient way to generate URLs such as https://example.com/.well-known/mercure?topic=https://example.com/books/{id}&topic=https://example.com/users/dunglas

const eventSource = new EventSource(url);

// The callback will be called every time an update is published
eventSource.onmessage = e => console.log(e); // do something with the payload
```

The `EventSource` class is available [in all modern web browsers](https://caniuse.com/#feat=eventsource). And for legacy browsers, [there are polyfills](resources.md#polyfills).

## Sending Private Updates

Optionally, [the authorization mechanism](../spec/mercure.md#authorization) can be used to subscribe to private updates.

![Authorization Schema](../spec/authorization.png)

## Discovering the Mercure Hub

Also optionally, the hub URL can be automatically discovered:

![Discovery Schema](../spec/discovery.png)

Here is a snippet to extract the URL of the hub from the `Link` HTTP header.

```javascript
fetch('https://example.com/books/1') // Has this header `Link: <https://example.com/.well-known/mercure>; rel="mercure"`
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
Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6WyJmb28iLCJiYXIiXSwicHVibGlzaCI6WyJmb28iXX19.afLx2f2ut3YgNVFStCx95Zm_UND1mZJ69OenXaDuZL8

topic=https://example.com/books/1&data={"foo": "updated value"}
```

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
    path: '/.well-known/mercure',
    method: 'POST',
    headers: {
        Authorization: 'Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6WyJmb28iLCJiYXIiXSwicHVibGlzaCI6WyJmb28iXX19.afLx2f2ut3YgNVFStCx95Zm_UND1mZJ69OenXaDuZL8',
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

The JWT must contain a `publish` property containing an array of targets. This array can be empty to allow publishing anonymous updates only. To create and read JWTs try [jwt.io](https://jwt.io) ([demo token](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6WyJmb28iLCJiYXIiXSwicHVibGlzaCI6WyJmb28iXX19.afLx2f2ut3YgNVFStCx95Zm_UND1mZJ69OenXaDuZL8), key: `!ChangeMe!`).

## Going Further

[Read the full specification](../spec/mercure.md)
