# Mercure: Server-Sent Live Updates
> A Protocol and Implementation to Subscribe to Data Updates (especially useful for Web APIs)

Mercure is a protocol and a reference implementation allowing servers to push live data updates to clients in a fast and
reliable way.
Is is especially useful to push real time updates of data served by web APIs, including [hypermedia APIs](https://en.wikipedia.org/wiki/HATEOAS).

A typical use case:

* a Progressive Web App queries a REST API to retrieve product's details, including its availability: only one is still
  available
* after some minutes, the product is bought by another customer
* we want the Progressive Web App to be instantly notified, and to refresh the view to show that this product isn't available
  anymore

**Mercure gets you covered!**

Client-side, it uses [Server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events),
thus is compatible with all modern browsers and can support older ones with a polyfill.
It requires no specific libraries or SDK to receive updates pushes.

Server-side, it is designed to be easy to use in any web application, including (but not limited to) the ones built using
serverless architectures (Amazon Lambda like), (Fast)CGI scripts and PHP.

Mercure also describes an authorization mechanism allowing to send updates only to a list of allowed clients.

A fast and easy to use reference implementation (written in Go) is provided in this repository.

## Protocol

### Terminology

* Publisher: a server that exposes resources to a subscriber (typically a website or a web API)
* Subscriber: a client application that subscribes to real time updates (typically a Progressive Web App or a Mobile
  App) 
* Hub: a server that handles subscription requests and distributes the content to subscribers when the corresponding resources
  have been updated (a Hub implementation is provided in this repository)

### Discovery

The publisher advertises the URL allowing the subscriber to receive live updates when resources are added, modified or deleted.
To advertise this URL, the publisher must uses [the discovery mechanism specified in the WebSub W3C recommendation](https://www.w3.org/TR/websub/#discovery),
but the name of the relation MUST be `mercure-hub` instead of `hub`.

Basically, the publisher will add a `Link` HTTP header with a `mercure-hub` relation to the generated HTTP response:

    GET /books/foo.jsonld HTTP/1.1
    Host: example.com

    HTTP/1.1 200 Ok
    Content-type: application/ld+json
    Link: <https://hub.example.com/updates/>; rel="mercure-hub"

    {"@id": "/books/foo.jsonld", "foo": "bar"}

Links embedded in HTML or XML documents (as defined in the WebSub recommendation) MAY also be supported by subscribers.

### Subscribtions

The subscriber subscribes to an URL exposed by a hub to receive updates of one or many resources.
To subscribe to updates, the client opens a HTTP (HTTPS and HTTP/2 are strongly recommended) connection following the [server-send
event specification](https://html.spec.whatwg.org/multipage/server-sent-events.html) to the hub's subscription URL advertised
by the Publisher.

It specifies the list of resources it wants to get updates for by using one or several query parameters named `iri[]`.
The value of these query parameters are [URI templates (RFC6570)](https://tools.ietf.org/html/rfc6570).

Note: an URL is also a valid URI template.

The hub sends updates concerning all subscribed resources matching the provided URI templates.
The hub MUST send these updates as [`text/event-stream` compliant events](https://html.spec.whatwg.org/multipage/server-sent-events.html#sse-processing-model).

* the `id` key of the event MUST contains the IRI of the resource being updated
* the `data` key MUST contain the new version of the resource
* the `event` key MUST be set to `mercure`, it allows a hub to mix Mercure events with other kind of Server-sent events

Example implementation of a client in JavaScript:

```javascript
const params = new URLSearchParams();
// The subscriber subscribes to updates for the https://example.com/foo resource
// and to any resource matching https://example.com/books/{name}
params.append('iri', 'https://example.com/foo');
params.append('iri', 'https://example.com/books/{name}');

const eventSource = new EventSource(`https://hub.example.com?${params}`);

// The following callaback will be called every time the Hub send an update 
eventSource.addEventListener('mercure', (e) => {
    console.log('Resource IRI: %s', e.lastEventId);
    console.log('Resource content: %s', e.data);
});
```

The protocol doesn't specify the maximum number of `iri[]` parameters that can be sent, but the hub MAY apply a limit.

### Hub

The hub receives updates from the publisher on a dedicated HTTP endpoint.
This endpoint MUST be exposed only trough a secure connection (HTTPS). 

WHen it receives an update, the hub dispatches it to subsribers using the established server-sent event connections.

This repository provides a full, high performance, implementation of a hub that can be used directly.

An application CAN send events directly to the subscribers, without using an external hub server, if it is able to do so.
In this case, it SHOULD not implement the endpoint to publish updates.
This repository also contain a Go library that can be used in Go application to implement Mercure.

The endpoint to publish updates is an HTTP URL accessed using the `POST` method.
The request must be encoded using the `application/x-www-form-urlencoded` and contains the following data:

* `iri`: the IRI of the updated resource
* `data`: the resources's content

The request MUST also contain an `Authorization` HTTP header containing the string `Bearer ` followed by a valid [JWT token
(RFC7519)](https://tools.ietf.org/html/rfc7519) that the hub will check to ensure that the publisher is authorized to publish
the update.

The connection MUST use an encryption layer, such as TLS. HTTPS certificate can be obtained for free using [Let's Encrypt](https://letsencrypt.org/).

## Authorization

If a resource is not public, the update request sent by the publisher to the hub MUST also contain a list of keys named `target[]`.
Theirs values are `string`. They can be, for instance a user ID, or a list of group IDs.

To receive updates for private resources, the subscriber must send a cookie called `mercure_authorization` when connecting
to the hub.

The value of this cookie is a JWT token that MUST be encrypted. It MUST contain a claim named `mercure_targets` and containing
an array of strings: the list of target the user is authorized to receive updates for.

If one or more targets are specified, the update MUST NOT be sent to the subscriber by the hub, unless the `mercure_targets`
claim of the subscriber contains at least one target specified for the resource by the publisher. 

When using the authorization mechanism, the connection between the subscriber and the hub MUST use an encryption layer (HTTPS
is required).

By the specification, server-sent events connection can only be executed with the `GET` HTTP method, and it is not possible to set
custom HTTP headers (such as the `Authorization` one).

However, cookies are supported, and can be included even in crossdomain requests if [the CORS credentials are set](https://html.spec.whatwg.org/multipage/server-sent-events.html#dom-eventsourceinit-withcredentials):

```javascript
const eventSource = new EventSource(`https://hub.example.com?${params}`, { withCredentials: true });
```

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
