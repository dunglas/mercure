# Mercure: Server-Sent Live Updates
> A Protocol and Implementation to Subscribe to Data Updates (especially useful for Web APIs)

Mercure is a protocol and a reference implementation allowing servers to push live data updates to clients in a fast,
reliable and battery-efficient way.
It is especially useful to push real-time updates of resources served through web APIs, including [hypermedia](https://en.wikipedia.org/wiki/HATEOAS) and [GraphQL](https://graphql.org/) APIs.

Typical use cases:

Availability display

* a Progressive Web App queries a REST API to retrieve product's details, including its availability: only one is still
  available
* after some minutes, the product is bought by another customer
* we want the Progressive Web App to be instantly notified, and to refresh the view to show that this product isn't available
  anymore

Collaborative editing

* a webapp allows several users to display and edit the same document through a form
* we want the webapp to instantly display changes made to the document by other users without requiring a page reload

**Mercure gets you covered!**

Client-side, it uses [Server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events),
thus is compatible with all modern browsers and can support older ones with a polyfill.
It requires **no specific libraries or SDK** to receive updates pushes.

Server-side, it is designed to be easy to use in any web application, including (but not limited to) the ones built using
serverless architectures (Amazon Lambda like), (Fast)CGI scripts and PHP.

Mercure also describes an authorization mechanism allowing to send updates only to a list of allowed clients.

A fast and easy to use reference implementation (written in Go) is provided in this repository.

## Protocol

### Terminology

* Topic: An HTTP [RFC7230](https://tools.ietf.org/html/rfc7230) (or HTTPS [RFC2818](https://tools.ietf.org/html/rfc2818)) topic URL. The unit to which one can subscribe to changes.
* Publisher: An owner of a topic. Notifies the hub when the topic feed has been updated. As in almost all pubsub systems, the publisher is unaware of the subscribers, if any. Other pubsub systems might call the publisher the "source". Typically a website or a web API.
* Subscriber: A client application that subscribes to real-time updates of topics (typically a Progressive Web App or a Mobile
  App).
* Hub: A server that handles subscription requests and distributes the content to subscribers when the corresponding topics
  have been updated (a Hub implementation is provided in this repository). Any hub MAY implement its own policies on who can use it.

### Discovery

The publisher SHOULD advertises the URL of one or more hubs to the subscriber, allowing it to receive live updates when topics are updated.
If more than one hub URL is specified, it is expected that the publisher notifies each hub, so the subscriber may subscribe to one or more of them.

The publisher SHOULD include at least one Link Header [RFC5988](https://tools.ietf.org/html/rfc5988) with `rel=mercure-hub` (a hub link header). The target URL of these links MUST be a hub implementing the Mercure protocol.

Note: this relation type has not been [registered](https://tools.ietf.org/html/rfc5988#section-6.2.1) yet. During the meantime, the relation type `https://git.io/mercure` can be used instead.

The publisher MAY provide the following target attributes in the Link headers:

* `jwt=<jwt-token>`: a valid JWT ([RFC7519](https://tools.ietf.org/html/rfc7519)) that will be used by the subscriber to be authorized by the hub (see the Authorization section). The JWT can also be transfered by the publisher to the subscriber using other off band channels including (but not limited too) OAuth 2.0 ([RFC6749](https://tools.ietf.org/html/rfc6749)).
* `jwe-key=<encryption-key>`: an encryption key to decode the JWE data (see the Encryption section).
* `content-type`: the content type of the updates that will pushed by the hub. If omited, the subscriber MUST assume that the content type will be the same than the one of the original resource.

Setting the `content-type` attribute is especially useful to hint that partial updates will be pushed, using formats such as JSON Patch ([RFC6902](https://tools.ietf.org/html/rfc6902)) or JSON Merge Patch ([RFC7386](https://tools.ietf.org/html/rfc7386)).

The publisher MAY also include one Link Header [RFC5988] with `rel=self` (the self link header). It SHOULD contain the canonical URL for the topic to which subscribers are expected to use for subscriptions. If the Link with `rel=self` is ommitted, the current URL of the resource MUST be used as fallback.

Minimal example:

    GET /books/foo.jsonld HTTP/1.1
    Host: example.com

    HTTP/1.1 200 Ok
    Content-type: application/ld+json
    Link: <https://hub.example.com/updates/>; rel="https://git.io/mercure"

    {"@id": "/books/foo.jsonld", "foo": "bar"}

Links embedded in HTML or XML documents (as defined in the WebSub recommendation) MAY also be supported by subscribers.

This discovery mechanism [is similar to the one specified in the WebSub recommendation](https://www.w3.org/TR/websub/#discovery).

### Subscriptions

The subscriber subscribes to an URL exposed by a hub to receive updates of one or many topics.
To subscribe to updates, the client opens an HTTPS connection following the [server-send
event specification](https://html.spec.whatwg.org/multipage/server-sent-events.html) to the hub's subscription URL advertised
by the Publisher.
The connection SHOULD use HTTP/2 to leverage mutliplexing and other advanced features of this protocol.

It specifies the list of topics it wants to get updates for by using one or several query parameters named `topic`.
The value of these query parameters MUST be [URI templates (RFC6570)](https://tools.ietf.org/html/rfc6570).

Note: an URL is also a valid URI template.

The hub sends updates concerning all subscribed resources matching the provided URI templates.
The hub MUST send these updates as [`text/event-stream` compliant events](https://html.spec.whatwg.org/multipage/server-sent-events.html#sse-processing-model).

The `data` property MUST contain the new version of the topic (it can be the full resource, or contain only the changes since the last version when using format such as JSON Patch or JSON Merge Patch)

All other properties defined in the Server-Sent Events specification MAY be used and SHOULD be supported by hubs.

The resource SHOULD be represented in a format with hypermedia capabilities such as [JSON-LD](https://www.w3.org/TR/json-ld/), [Atom](https://tools.ietf.org/html/rfc4287), [XML](https://www.w3.org/XML/) or [HTML](https://html.spec.whatwg.org/).

[Web Linking](https://tools.ietf.org/html/rfc5988) SHOULD be used to indicate the IRI of the resource sent in the event.
When using Atom, XML or HTML as serialization format for the resource, the document SHOULD contain a `link` element with a `self` relation containing the IRI of the resource.
When using JSON-LD, the document SHOULD contain an `@id` property containing the IRI of the resource.

The hub SHOULD support the other [optional capabilities defined in the Server Sent Event specification](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events).

Namely, events MAY include an `id` key.
The value associated to this `id` property SHOULD be a globally unique identifier.
Using an [UUID](https://tools.ietf.org/html/rfc4122) is RECOMMENDED.

According to the Server Sent Event specification, in case of connection lost the subscriber will try to automatically reconnect. During the reconnection the subscriber will send the last received event id in a [`Last-Event-Id`](https://html.spec.whatwg.org/multipage/iana.html#last-event-id) HTTP header .
If such header exists, the hub MAY send to the subscriber all events published since the one having this identifier.
It's a the hub CAN discard some messages for operational reasons. The subscriber MUST NOT assume that no message will be lost, and MUST re-fetch the original topic to ensure this (for instance, after a long deconnection).

The hub MAY also specify the reconnection time using the `retry` key.

Example implementation of a client in JavaScript:

```javascript
const params = new URLSearchParams();
// The subscriber subscribes to updates for the https://example.com/foo topic
// and to any topic matching https://example.com/books/{name}
params.append('topic', 'https://example.com/foo');
params.append('topic', 'https://example.com/books/{name}');

const eventSource = new EventSource(`https://hub.example.com?${params}`);

// The following callaback will be called every time the Hub send an update 
eventSource.on message = function ({data}) {
    console.log(data);
};
```

The protocol doesn't specify the maximum number of `topic` parameters that can be sent, but the hub MAY apply a limit.

### Hub

The hub receives updates from the publisher on a dedicated HTTPS endpoint.
The connection MUST use an encryption layer, such as TLS. HTTPS certificate can be obtained for free using [Let's Encrypt](https://letsencrypt.org/).

When it receives an update, the hub dispatches it to subsribers using the established server-sent event connections.

Note: This repository provides a full, high performance, implementation of a hub that can be used directly.

An application CAN send events directly to the subscribers, without using an external hub server, if it is able to do so.
In this case, it MAY not implement the endpoint to publish updates.

Note: This repository also contains a library that can be used in Go application to implement the Mercure protocol.

The endpoint to publish updates is an HTTPS URL accessed using the `POST` method.
The request must be encoded using the `application/x-www-form-urlencoded` format and contains the following data:

* `topic`: IRIs of the updated topic. If this key is present several times, the first occurence is considered to be the canonical URL of the topic, and other ones are considered to be alternate URLs. The hub must dispatch this update to subscribers subscribed to both canonical or alternate URLs.
* `data`: the content of the new version of this topic
* `target` (optional): target audience of this event, see the Authorization section for further information.
* `id` (optional): the topic's revision identifier, it will be used as the SSE's `id` property, if omited the hub MUST generate a valid UUID.
* `type` (optional): the SSE's `event` property (a specific event type)
* `retry` (optional): the SSE's `retry` property (the reconnection time)

The request MUST also contain an `Authorization` HTTP header containing the string `Bearer ` followed by a valid [JWT token
(RFC7519)](https://tools.ietf.org/html/rfc7519) that the hub will check to ensure that the publisher is authorized to publish
the update.

## Authorization

If a topic is not public, the update request sent by the publisher to the hub MUST also contain a list of keys named `target[]`.
Theirs values are `string`. They can be, for instance a user ID, or a list of group IDs.

To receive updates for private topics, the subscriber must send a cookie called `mercureAuthorization` when connecting
to the hub.

The value of this cookie MUST be a JWT token. It MUST have a claim named `mercureTargets` and containing
an array of strings: the list of target the user is authorized to receive updates for.

If one or more targets are specified, the update MUST NOT be sent to the subscriber by the hub, unless the `mercureTargets`
claim of the subscriber contains at least one target specified for the topic by the publisher. 

When using the authorization mechanism, the connection between the subscriber and the hub MUST use an encryption layer (HTTPS
is required).

By the specification, server-sent events connection can only be executed with the `GET` HTTP method, and it is not possible to set
custom HTTP headers (such as the `Authorization` one).

However, cookies are supported, and can be included even in crossdomain requests if [the CORS credentials are set](https://html.spec.whatwg.org/multipage/server-sent-events.html#dom-eventsourceinit-withcredentials):

```javascript
const eventSource = new EventSource(`https://hub.example.com?${params}`, { withCredentials: true });
```

## The Hub Implementation

Environment variables:

* `PUBLISHER_JWT_KEY`: must contain the secret key to valid publishers' JWT
* `SUBSCRIBER_JWT_KEY`: must contain the secret key to valid subscribers' JWT
* `CORS_ALLOWED_ORIGINS`: must contain a comma separated list of hosts allowed to subscribe
* `ALLOW_ANONYMOUS`:  set to `1` to allow subscribers with no valid JWT to connect
* `DEBUG`: set to `1` to enable the debug mode (prints recovery stack traces)
* `DEMO`: set to `1` to enable the demo mode (automatically enabled when `DEBUG=1`)

## Contributing

Clone the project:

    $ git clone https://github.com/dunglas/mercure
    
Install Gin to get the Live Reloading:

    $ go get github.com/codegangsta/gin

Install the dependencies:

    $ cd mercure
    $ dep ensure

Run the server:

    $ gin run main.go

Go to `http://localhost:3001` and enjoy!
