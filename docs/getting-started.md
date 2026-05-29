# Getting Started

![Subscriptions Schema](../spec/subscriptions.png)

## Starting the Hub

The easiest way to get started is to [install the official Mercure.rocks Hub](hub/install.md). Once installed, proceed to the next step.
There are also unofficial [libraries implementing Mercure](ecosystem/awesome.md#hubs-and-server-libraries). In the rest of this tutorial, we'll assume the hub is running on `https://localhost` and that the `JWT_KEY` is `!ChangeThisMercureHubJWTSecretKey!`.

Note: The hub is entirely optional when using the Mercure protocol. Your app can also implement the Mercure protocol directly.

## Subscribing

Subscribing to updates from a web browser or any other platform supporting [Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events) is straightforward:

```javascript
// The subscriber subscribes to updates for the https://example.com/users/dunglas topic
// and to any URL matching the https://example.com/books/:id URL pattern.
const url = new URL("https://localhost/.well-known/mercure");
url.searchParams.append("match", "https://example.com/users/dunglas");
url.searchParams.append("matchURLPattern", "https://example.com/books/:id");
// The URL class is a convenient way to generate URLs such as
// https://localhost/.well-known/mercure?match=https://example.com/users/dunglas&matchURLPattern=https://example.com/books/:id

const eventSource = new EventSource(url);

// The callback will be called every time an update is published
eventSource.onmessage = (e) => console.log(e); // do something with the payload
```

The `match` query parameter performs a case-sensitive exact comparison.
For pattern matching, use `matchURLPattern` ([URL Patterns](https://urlpattern.spec.whatwg.org/)),
`matchRegexp` ([I-Regexp, RFC 9485](https://www.rfc-editor.org/rfc/rfc9485.html)),
`matchURITemplate` ([URI Templates, RFC 6570](https://tools.ietf.org/html/rfc6570))
or `matchCEL` ([CEL](https://github.com/google/cel-spec)). Use the reserved
string `*` as the `match` value to subscribe to every topic.

The `EventSource` class is available [in all modern web browsers](https://caniuse.com/eventsource).

Although the native `EventSource` class is generally quite good, we recommend [Microsoft's `fetch-event-source` library](https://github.com/Azure/fetch-event-source) for advanced use cases, as it allows finer-grained error handling and supports authentication via the `Authorization` header.

## Closing Connection

It is important to close the connection between the client and the hub if it is no longer needed.
Open connections maintain a continuous buffer that can drain your application resources.
This is especially true when using Single Page Applications (e.g., React): the connection is maintained even if the component that created it is unmounted.

To close the connection, call `eventSource.close()`.

## Sending Private Updates

Optionally, [the authorization mechanism](../spec/mercure.md#authorization) can be used to subscribe to private updates.

![Authorization Schema](../spec/authorization.png)

## Discovering the Mercure Hub

Optionally, the hub URL can be automatically discovered:

![Discovery Schema](../spec/discovery.png)

Here is a snippet to extract the URL of the hub from the `Link` HTTP header:

```javascript
fetch("https://example.com/books/1") // Has this header: `Link: <https://localhost/.well-known/mercure>; rel="mercure"`
  .then((response) => {
    // Extract the hub URL from the Link header
    const hubUrl = response.headers
      .get("Link")
      .match(/<([^>]+)>;\s+rel=(?:mercure|"[^"]*mercure[^"]*")/)[1];
    // Subscribe to updates using the first snippet, do something with response's body...
  });
```

## Publishing

To dispatch an update, the publisher (an application server, a web browser...) needs to send a `POST` HTTP request to the hub:

```http
POST example.com HTTP/1.1
Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlt7Im1hdGNoIjoiKiJ9XSwic3Vic2NyaWJlIjpbeyJtYXRjaCI6Imh0dHBzOi8vZXhhbXBsZS5jb20vbXktcHJpdmF0ZS10b3BpYyJ9LHsibWF0Y2giOiJodHRwczovL2V4YW1wbGUuY29tL2RlbW8vYm9va3MvOmlkLmpzb25sZCIsIm1hdGNoVHlwZSI6IlVSTFBhdHRlcm4ifSx7Im1hdGNoIjoiLy53ZWxsLWtub3duL21lcmN1cmUvc3Vic2NyaXB0aW9ucy86bWF0Y2hUeXBlLzptYXRjaC86c3Vic2NyaWJlciIsIm1hdGNoVHlwZSI6IlVSTFBhdHRlcm4ifV0sInBheWxvYWQiOnsicmVtb3RlQWRkciI6IjEyNy4wLjAuMSIsInVzZXIiOiJodHRwczovL2V4YW1wbGUuY29tL3VzZXJzL2R1bmdsYXMifX19.-I_LuyEjpjZKSfFI-4BstvrLzdCNslsSjHfR5RX0PcM

topic=https://example.com/books/1&data={"foo": "updated value"}
```

Example using [cURL](https://curl.haxx.se/):

```bash
curl -d 'topic=https://example.com/books/1' -d 'data={"foo": "updated value"}' -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlt7Im1hdGNoIjoiKiJ9XSwic3Vic2NyaWJlIjpbeyJtYXRjaCI6Imh0dHBzOi8vZXhhbXBsZS5jb20vbXktcHJpdmF0ZS10b3BpYyJ9LHsibWF0Y2giOiJodHRwczovL2V4YW1wbGUuY29tL2RlbW8vYm9va3MvOmlkLmpzb25sZCIsIm1hdGNoVHlwZSI6IlVSTFBhdHRlcm4ifSx7Im1hdGNoIjoiLy53ZWxsLWtub3duL21lcmN1cmUvc3Vic2NyaXB0aW9ucy86bWF0Y2hUeXBlLzptYXRjaC86c3Vic2NyaWJlciIsIm1hdGNoVHlwZSI6IlVSTFBhdHRlcm4ifV0sInBheWxvYWQiOnsicmVtb3RlQWRkciI6IjEyNy4wLjAuMSIsInVzZXIiOiJodHRwczovL2V4YW1wbGUuY29tL3VzZXJzL2R1bmdsYXMifX19.-I_LuyEjpjZKSfFI-4BstvrLzdCNslsSjHfR5RX0PcM' -X POST https://localhost/.well-known/mercure
```

Example using [Node.js](https://nodejs.org/) / [Serverless](https://serverless.com/):

```javascript
// Handle a POST, PUT, PATCH or DELETE request or finish an async job...
// and notify the hub
const http = require("http");
const querystring = require("querystring");

const postData = querystring.stringify({
  topic: "https://example.com/books/1",
  data: JSON.stringify({ foo: "updated value" }),
});

const req = http.request(
  {
    hostname: "localhost",
    port: "3000",
    path: "/.well-known/mercure",
    method: "POST",
    headers: {
      Authorization:
        "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlt7Im1hdGNoIjoiKiJ9XSwic3Vic2NyaWJlIjpbeyJtYXRjaCI6Imh0dHBzOi8vZXhhbXBsZS5jb20vbXktcHJpdmF0ZS10b3BpYyJ9LHsibWF0Y2giOiJodHRwczovL2V4YW1wbGUuY29tL2RlbW8vYm9va3MvOmlkLmpzb25sZCIsIm1hdGNoVHlwZSI6IlVSTFBhdHRlcm4ifSx7Im1hdGNoIjoiLy53ZWxsLWtub3duL21lcmN1cmUvc3Vic2NyaXB0aW9ucy86bWF0Y2hUeXBlLzptYXRjaC86c3Vic2NyaWJlciIsIm1hdGNoVHlwZSI6IlVSTFBhdHRlcm4ifV0sInBheWxvYWQiOnsicmVtb3RlQWRkciI6IjEyNy4wLjAuMSIsInVzZXIiOiJodHRwczovL2V4YW1wbGUuY29tL3VzZXJzL2R1bmdsYXMifX19.-I_LuyEjpjZKSfFI-4BstvrLzdCNslsSjHfR5RX0PcM",
      // the JWT must have a mercure.publish key containing an array of topic matcher objects
      // (can contain {"match":"*"} for all topics, and be empty for public updates).
      // the JWT key must be shared between the hub and the server.
      "Content-Type": "application/x-www-form-urlencoded",
      "Content-Length": Buffer.byteLength(postData),
    },
  } /* optional response handler */,
);
req.write(postData);
req.end();

// You'll probably prefer use the request library or the node-fetch polyfill in real projects,
// but any HTTP client, written in any language, will be just fine.
```

The JWT must contain a `publish` property containing an array of topic matcher
objects (`{"match": "<pattern>", "matchType": "Exact|URLPattern|Regexp|URITemplate|CEL"}`;
`matchType` defaults to `Exact` when omitted). The array can be empty to allow
publishing anonymous updates only. Use `{"match": "*"}` to allow publishing
updates for every topic. To create and read JWTs try [jwt.io](https://jwt.io) ([demo token](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlt7Im1hdGNoIjoiKiJ9XSwic3Vic2NyaWJlIjpbeyJtYXRjaCI6Imh0dHBzOi8vZXhhbXBsZS5jb20vbXktcHJpdmF0ZS10b3BpYyJ9LHsibWF0Y2giOiJodHRwczovL2V4YW1wbGUuY29tL2RlbW8vYm9va3MvOmlkLmpzb25sZCIsIm1hdGNoVHlwZSI6IlVSTFBhdHRlcm4ifSx7Im1hdGNoIjoiLy53ZWxsLWtub3duL21lcmN1cmUvc3Vic2NyaXB0aW9ucy86bWF0Y2hUeXBlLzptYXRjaC86c3Vic2NyaWJlciIsIm1hdGNoVHlwZSI6IlVSTFBhdHRlcm4ifV0sInBheWxvYWQiOnsicmVtb3RlQWRkciI6IjEyNy4wLjAuMSIsInVzZXIiOiJodHRwczovL2V4YW1wbGUuY29tL3VzZXJzL2R1bmdsYXMifX19.-I_LuyEjpjZKSfFI-4BstvrLzdCNslsSjHfR5RX0PcM), key: `!ChangeThisMercureHubJWTSecretKey!`).

## Active Subscriptions

Mercure dispatches events every time a new subscription is created or terminated. It also exposes a web API to retrieve the list of active subscriptions.

[Learn more about subscriptions](../spec/mercure.md#active-subscriptions).

## Going Further

- [Read the full specification](../spec/mercure.md)
- [Install the hub](hub/install.md)
- [Checkout the examples](ecosystem/awesome.md#examples)
