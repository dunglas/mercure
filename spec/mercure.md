%%%
title = "The Mercure Protocol"
area = "Internet"
workgroup = "Network Working Group"

[seriesInfo]
name = "Internet-Draft"
value = "draft-dunglas-mercure-05"
stream = "IETF"
status = "informational"

[[author]]
initials="K."
surname="Dunglas"
fullname="Kévin Dunglas"
abbrev = "Les-Tilleuls.coop"
organization = "Les-Tilleuls.coop"
  [author.address]
  email = "kevin@les-tilleuls.coop"
  [author.address.postal]
  city = "Lille"
  street = "82 rue Winston Churchill"
  code = "59160"
  country = "France"
%%%

.# Abstract

Mercure is a protocol enabling the pushing of data updates to web browsers and other HTTP clients in
a fast, reliable and battery-efficient way. It is especially useful for publishing real-time updates
of resources served through web APIs to web and mobile apps.

{mainmatter}

# Terminology

The keywords **MUST**, **MUST NOT**, **REQUIRED**, **SHALL**, **SHALL NOT**, **SHOULD**, **SHOULD
NOT**, **RECOMMENDED**, **MAY**, and **OPTIONAL**, when they appear in this document, are to be
interpreted as described in [@!RFC2119].

 *  Topic: The unit to which one can subscribe to changes. The topic **SHOULD** be identified
    by an IRI [@!RFC3987]. Using an HTTPS [@!RFC7230] or HTTP [@!RFC7230] URI [@!RFC3986] is
    **RECOMMENDED**.

 *  Update: The message containing the updated version of the topic. An update can be marked as
    private, consequently, it must be dispatched only to subscribers allowed to receive it.

 *  Topic selector: An expression matching one or several topics.

 *  Publisher: An owner of a topic. Notifies the hub when the topic feed has been updated. As in
    almost all pubsub systems, the publisher is unaware of the subscribers, if any. Other pubsub
    systems might call the publisher the "source". Typically a website or a web API, but can also be
    a web browser.

 *  Subscriber: A client application that subscribes to real-time updates of topics using topic
    selectors. Typically a web or a mobile application, but can also be a server.

 *  Subscription: A topic selector used by a subscriber to receive updates. A single subscriber can
    have several subscriptions, when it provides several topic selectors.

 *  Hub: A server that handles subscription requests and distributes the content to subscribers when
    the corresponding topics have been updated. Any hub **MAY** implement its own policies on who
    can use it.

# Discovery

The discovery mechanism aims at identifying at least 2 URLs.

1.  The URL of one or more hubs designated by the publisher.

2.  The canonical URL for the topic to which subscribers are expected to use for subscriptions.

The URL of the hub **MUST** be the "well-known" [@!RFC5785] fixed path `/.well-known/mercure`.

If the publisher is a server, it **SHOULD** advertise the URL of one or more hubs to the subscriber,
allowing it to receive live updates when topics are updated. If more than one hub URL is specified,
the publisher **MUST** notifies each hub, so the subscriber **MAY** subscribe to one or more of
them.

Note: Publishers may wish to advertise and publish to more than one hub for fault tolerance and
redundancy. If one hub fails to propagate an update to the document, then using multiple independent
hub is a way to increase the likelihood of delivery to subscribers. As such, subscribers may
subscribe to one or more of the advertised hubs.

The publisher **SHOULD** include at least one Link Header [@!RFC5988] with `rel=mercure` (a hub link
header). The target URL of these links **MUST** be a hub implementing the Mercure protocol.

The publisher **MAY** provide the following target attributes in the Link Headers:

 *  `last-event-id`: the identifier of the last event dispatched by the publisher at the time of
    the generation of this resource. If provided, it **MUST** be passed to the hub through a query
    parameter called `Last-Event-ID` and will be used to ensure that possible updates having been
    made during between the resource generation time and the connection to the hub are not lost. See
    (#reconciliation).

 *  `content-type`: the content type of the updates that will pushed by the hub. If omitted, the
    subscriber **MUST** assume that the content type will be the same as that of the original
    resource. Setting the `content-type` attribute is especially useful to hint that partial updates
    will be pushed, using formats such as JSON Patch [@RFC6902] or JSON Merge Patch [@RFC7386].

 *  `key-set`: the URL of the key set to use to decrypt updates, encoded in the JWK set format
    (JSON Web Key Set) [@!RFC7517]. See (#encryption). As this key set will contain a secret
    key, the publisher must ensure that only the subscriber can access to this URL. To do so, the
    authorization mechanism (see (#authorization)) can be reused.

All these attributes are optional.

The publisher **MAY** also include one Link Header [@!RFC5988] with `rel=self` (the self link
header). It **SHOULD** contain the canonical URL for the topic to which subscribers are expected
to use for subscriptions. If the Link with `rel=self` is omitted, the current URL of the resource
**MUST** be used as a fallback.

Minimal example:

~~~ http
GET /books/foo HTTP/1.1
Host: example.com

HTTP/1.1 200 OK
Content-type: application/ld+json
Link: <https://example.com/.well-known/mercure>; rel="mercure"

{"@id": "/books/foo", "foo": "bar"}
~~~

Links embedded in HTML or XML documents as defined in the WebSub recommendation
[@W3C.REC-websub-20180123] **MAY** also be supported by subscribers. If both a header and an
embedded link are provided, the header **MUST** be preferred.

## Content Negotiation

For practical purposes, it is important that the `rel=self` URL only offers a single representation.
As the hub has no way of knowing what Media Type ([@RFC6838]) or language may have been requested
by the subscriber upon discovery, it would not be able to deliver the content using the appropriate
representation of the document.

It is, however, possible to perform content negotiation by returning an appropriate `rel=self`
URL according to the HTTP headers used in the initial discovery request. For example, a request
to `/books/foo` with an `Accept` header containing `application/ld+json` could return a `rel=self`
value of `/books/foo.jsonld`.

The example below illustrates how a topic URL can return different `Link` headers depending on the
`Accept` header that was sent.

~~~ http
GET /books/foo HTTP/1.1
Host: example.com
Accept: application/ld+json

HTTP/1.1 200 OK
Content-type: application/ld+json
Link: </books/foo.jsonld>; rel="self"
Link: <https://example.com/.well-known/mercure>; rel="mercure"

{"@id": "/books/foo", "foo": "bar"}
~~~

~~~ http
GET /books/foo HTTP/1.1
Host: example.com
Accept: text/html

HTTP/1.1 200 OK
Content-type: text/html
Link: </books/foo.html>; rel="self"
Link: <https://example.com/.well-known/mercure>; rel="mercure"

<!doctype html>
<title>foo: bar</title>
~~~

Similarly, the technique can also be used to return a different `rel=self` URL depending on the
language requested by the `Accept-Language` header.

~~~ http
GET /books/foo HTTP/1.1
Host: example.com
Accept-Language: fr-FR

HTTP/1.1 200 OK
Content-type: application/ld+json
Content-Language: fr-FR
Link: </books/foo-fr-FR.jsonld>; rel="self"
Link: <https://example.com/.well-known/mercure>; rel="mercure"

{"@id": "/books/foo", "foo": "bar", "@context": {"@language": "fr-FR"}}
~~~

# Topic Selectors

A topic selector is an expression intended to be matched by one or several topics. A topic selector
can also be used to match other topic selectors for authorization purposes. See (#authorization).

A topic selector can be a any string including URI Templates [@!RFC6570] and the reserved string `*`
that matches all topics. It is **RECOMMENDED** to use URI Templates or the reserved string `*` as
topic selectors.

Note: URLs and IRIs are valid URI templates.

To determine if a string matches a selector, the following steps must be followed:

1.  If the topic selector is `*` then the string matches the selector.

2.  If the topic selector and the string are exactly the same, the string matches the selector. This
    characteristic allows to compare a URI Template with another one.

3.  If the topic selector is a valid URI Template, and that the string matches this URI Template,
    the string matches the selector.

4.  Otherwise the string does not match the selector.

# Subscription

The subscriber subscribes to a URL exposed by a hub to receive updates from one or many topics.
To subscribe to updates, the client opens an HTTPS connection following the Server-Sent Events
specification [@!W3C.REC-eventsource-20150203] to the hub's subscription URL advertised by the
publisher. The `GET` HTTP method must be used. The connection **SHOULD** use HTTP/2 to leverage
mutliplexing and other advanced features of this protocol.

The subscriber specifies the list of topics to get updates from by using one or several query
parameters named `topic`. The `topic` query parameters **MUST** contain topic selectors. See
(#topic-selectors).

The protocol doesn't specify the maximum number of `topic` parameters that can be sent, but the hub
**MAY** apply an arbitrary limit. A subscription is created for every provided `topic` parameter.
See (#subscription-events).

[The EventSource JavaScript
interface](https://html.spec.whatwg.org/multipage/server-sent-events.html#the-eventsource-interface)
**MAY** be used to establish the connection. Any other appropriate mechanism
including, but not limited to, readable streams [@W3C.NOTE-streams-api-20161129] and
[XMLHttpRequest](https://xhr.spec.whatwg.org/) (used by popular polyfills) **MAY** also be used.

The hub sends to the subscriber updates for topics matching the provided topic selectors.

If an update is marked as `private`, the hub **MUST NOT** dispatch it to subscribers not authorized
to receive it. See (#authorization).

The hub **MUST** send these updates as `text/event-stream` compliant events
[!@W3C.REC-eventsource-20150203].

The `data` property **MUST** contain the new version of the topic. It can be the full resource, or a
partial update by using formats such as JSON Patch [@RFC6902] or JSON Merge Patch [@RFC7386].

All other properties defined in the Server-Sent Events specification **MAY** be used and **MUST** be
supported by hubs.

The resource **SHOULD** be represented in a format with hypermedia capabilities such as
JSON-LD [@W3C.REC-json-ld-20140116], Atom [@RFC4287], XML [@W3C.REC-xml-20081126] or HTML
[@W3C.REC-html52-20171214].

Web Linking [@!RFC5988] **SHOULD** be used to indicate the IRI of the resource sent in the event.
When using Atom, XML or HTML as the serialization format for the resource, the document **SHOULD**
contain a `link` element with a `self` relation containing the IRI of the resource. When using
JSON-LD, the document **SHOULD** contain an `@id` property containing the IRI of the resource.

Example:

~~~ javascript
// The subscriber subscribes to updates
// for the https://example.com/foo topic, the bar topic,
// and to any topic matching https://example.com/books/{name}
const url = new URL('https://example.com/.well-known/mercure');
url.searchParams.append('topic', 'https://example.com/foo');
url.searchParams.append('topic', 'bar');
url.searchParams.append('topic', 'https://example.com/bar/{id}');

const eventSource = new EventSource(url);

// The callback will be called every time an update is published
eventSource.onmessage = function ({data}) {
    console.log(data);
};
~~~

The hub **MAY** require subscribers and publishers to be authenticated, and **MAY** apply extra
authorization rules not defined in this specification.

# Publication

The publisher sends updates by issuing `POST` HTTPS requests on the hub URL. When it receives an
update, the hub dispatches it to subscribers using the established server-sent events connections.

An application **CAN** send events directly to subscribers without using an external hub server, if
it is able to do so. In this case, it **MAY NOT** implement the endpoint to publish updates.

The request **MUST** be encoded using the `application/x-www-form-urlencoded` format
[@W3C.REC-html52-20171214] and contains the following name-value tuples:

 *  `topic`: The identifiers of the updated topic. It is **RECOMMENDED** to use an IRI as
    identifier. If this name is present several times, the first occurrence is considered to be the
    canonical IRI of the topic, and other ones are considered to be alternate IRIs. The hub **MUST**
    dispatch this update to subscribers that are subscribed to both canonical or alternate IRIs.

 *  `data` (optional): the content of the new version of this topic.

 *  `private` (optional): if this name is set, the update **MUST NOT** be dispatched to subscribers
    not authorized to receive it. See (#authorization). It is recommended to set the value to `on`
    but it **CAN** contain any value including an empty string.

 *  `id` (optional): the topic's revision identifier: it will be used as the SSE's `id` property.
    If omitted, the hub **MUST** generate a valid IRI [@!RFC3987]. An UUID [@RFC4122] or a DID
    (@W3C.WD-did-core-20200421) **MAY** be used. Even if provided, the hub **MAY** ignore the id
    provided by the client and generate its own id.

 *  `type` (optional): the SSE's `event` property (a specific event type).

 *  `retry` (optional): the SSE's `retry` property (the reconnection time).

In the event of success, the HTTP response's body **MUST** be the `id` associated to this update
generated by the hub and a success HTTP status code **MUST** be returned. The publisher **MUST** be
authorized to publish updates. See (#authorization).

# Authorization

To ensure that they are authorized, both publishers and subscribers must present a valid JWS
[@!RFC7515] in compact serialization to the hub. This JWS **SHOULD** be short-lived, especially
if the subscriber is a web browser. A different key **MAY** be used to sign subscribers' and
publishers' tokens.

Two mechanisms are defined to present the JWS to the hub:

 *  using an `Authorization` HTTP header

 *  using a cookie

If the publisher or the subscriber is not a web browser, it **SHOULD** use an `Authorization`
HTTP header. This `Authorization` header **MUST** contain the string `Bearer` followed by the JWS.
The hub will check that the JWS conforms to the rules (defined later) ensuring that the client is
authorized to publish or subscribe to updates.

By the `EventSource` specification [@W3C.REC-eventsource-20150203], web browsers
can not set custom HTTP headers for such connections, and they can only be
established using the `GET` HTTP method. However, cookies are supported and
can be included even in cross-domain requests if [the CORS credentials are
set](https://html.spec.whatwg.org/multipage/server-sent-events.html#dom-eventsourceinit-withcredentials):

If the publisher or the subscriber is a web browser, it **SHOULD** send a cookie called
`mercureAuthorization` containing the JWS when connecting to the hub.

Whenever possible, the `mercureAuthorization` cookie **SHOULD** be set during discovery (see
(#discovery)) to improve the overall security. Consequently, if the cookie is set during the
discovery, both the publisher and the hub have to share the same second level domain. The `Domain`
attribute **MAY** be used to allow the publisher and the hub to use different subdomains. See
(#discovery).

The cookie **SHOULD** have the `Secure`, `HttpOnly` and `SameSite` attributes set. The cookie's
`Path` attribute **SHOULD** also be set to the hub's URL. See (#security-considerations).

When using authorization mechanisms, the connection **MUST** use an encryption layer such as HTTPS.

If both an `Authorization` HTTP header and a cookie named `mercureAuthorization` are presented by
the client, the cookie **MUST** be ignored. If the client tries to execute an operation it is not
allowed to, a 403 HTTP status code **SHOULD** be returned.

## Publishers

Publishers **MUST** be authorized to dispatch updates to the hub, and **MUST** prove that they are
authorized to send updates for the specified topics.

To be allowed to publish an update, the JWS presented by the publisher **MUST** contain a claim
called `mercure`, and this claim **MUST** contain a `publish` key. `mercure.publish` contains an
array of topic selectors. See (#topic-selectors).

If `mercure.publish`:

 *  is not defined, then the publisher **MUST NOT** be authorized to dispatch any update

 *  contains an empty array, the publisher **MUST NOT** be authorized to publish private updates,
    but can publish public updates for all topics.

Otherwise, the hub **MUST** check that every topics of the update to dispatch matches at least one
of the topic selectors contained in `mercure.publish`.

If the publisher is not authorized for all the topics of an update, the hub **MUST NOT** dispatch
the update (even if some topics in the list are allowed) and **MUST** return a 403 HTTP status code.

## Subscribers

To receive updates marked as `private`, a subscriber **MUST** prove that it is authorized for at
least one of the topics of this update. If the subscriber is not authorized to receive an update
marked as `private`, it **MUST NOT** receive it.

To receive updates marked as `private`, the JWS presented by the subscriber **MUST** have a
claim named `mercure` with a key named `subscribe` that contains an array of topic selectors. See
(#topic-selectors).

The hub **MUST** check that at least one topic of the update to dispatch matches at least one topic
selector provided in `mercure.subscribe`.

## Payload

The `mercure` claim of the JWS **CAN** also contain user-defined values under the `payload` key.
This JSON document will be attached to the subscription and made available in subscription events.
See (#subscription-events).

For instance, `mercure.payload` can contain the user ID of the subscriber, a list of groups it
belongs to, or its IP address. Storing data in `mercure.payload` is a convenient way to share data
related to one subscriber to other subscribers.

# Reconnection, State Reconciliation and Event Sourcing {#reconciliation}

The protocol allows to reconciliate states after a reconnection. It can also be used to implement an
[Event store](https://en.wikipedia.org/wiki/Event_store).

To allow re-establishment in case of connection lost, events dispatched by the hub **MUST** include
an `id` property. The value contained in this `id` property **SHOULD** be an IRI [@!RFC3987]. An
UUID [@RFC4122] or a DID (@W3C.WD-did-core-20200421) **MAY** be used.

According to the server-sent events specification, in case of connection
lost the subscriber will try to automatically re-connect. During the
re-connection, the subscriber **MUST** send the last received event id in a
[Last-Event-ID](https://html.spec.whatwg.org/multipage/iana.html#last-event-id) HTTP header.

In order to fetch any update dispatched between the initial resource generation by the publisher and
the connection to the hub, the subscriber **MUST** send the event id provided during the discovery
in the `last-event-id` as the last event id. See (#discovery).

`EventSource` implementations may not allow to set HTTP headers during the first connection (before
a reconnection) and implementations in web browsers don't allow to set it.

To work around this problem, the hub **MUST** also allow to pass the last event id in a query
parameter named `Last-Event-ID`.

If both the `Last-Event-ID` HTTP header and the query parameter are present, the HTTP header
**MUST** take precedence.

If the `Last-Event-ID` HTTP header or query parameter exists, the hub **SHOULD** send all events
published following the one bearing this identifier to the subscriber.

The reserved value `earliest` can be used to hint the hub to send all updates it has for the
subscribed topics. According to its own policy, the hub **MAY** or **MAY NOT** fulfil this request.

The hub **MAY** discard some events for operational reasons. If the hub is not able to send all
requested events, it **MUST** set a `Last-Event-ID` header on the HTTP response containing the id of
event preceding the first sent to the subscriber. If such event doesn't exist, the hub **MUST** set
the `Last-Event-ID` header it sends to the reserved value `earliest`. This value indicates that all
events stored for the subscribed topics have been sent to the subscriber.

The subscriber **MUST NOT** assume that no events will be lost (it may happen, for example after
a long disconnection time). In some cases (for instance when sending partial updates in the JSON
Patch [@RFC6902] format, or when using the hub as an event store), updates lost can cause data lost.
To check if a data lost ocurred, the subscriber **CAN** check if the requested last event id and
the value of the received `Last-Event-ID` match. In case of data lost, the subscriber **SHOULD**
re-fetch the original topic.

Note: Native `EventSource` implementations don't give access to headers associated with the HTTP
response, however polyfills and server-sent events clients in most programming languages allow it.

The hub **CAN** also specify the reconnection time using the `retry` key, as specified in the
server-sent events format.

# Active Subscriptions

Mercure provides a mechanism to track active subscriptions. If the hub support this optional set
of features, updates will be published when a subscription is created, or terminated, and a web API
exposes the list of active subscriptions.

Variables are templated and expanded in accordance with [@!RFC6570].

## Subscription Events

If the hub supports the active subscription feature, it **MUST** publish an update when a
subscription is created or terminated. If this feature is implemented by the hub, an update **MUST**
be dispatched every time that a subscription is created or terminated.

The topic of these updates **MUST** be an expansion of
`/.well-known/mercure/subscriptions/{topic}/{subscriber}`. `{topic}` is the topic selector used for
this subscription and `{subscriber}` is an unique identifier for the subscriber.

Note: Because it is recommended to use URI Templates and IRIs for the `{topic}` and `{subscriber}`
variables, values will usually contain the `:`, `/`, `{` and `}` characters. Per [@!RFC6570], these
characters are reserved. They **MUST** be percent encoded during the expansion process.

If a subscriber has several subscriptions, it **SHOULD** be identified by the same
identifier. `{subscriber}` **SHOULD** be an IRI [@!RFC3987]. An UUID [@RFC4122] or a DID
(@W3C.WD-did-core-20200421) **MAY** be used.

The content of the update **MUST** be a JSON-LD [@!W3C.REC-json-ld-20140116] document containing at
least the following properties:

 *  `@context`: the fixed value `https://mercure.rocks/`. `@context` can be omitted if already
    defined in a parent node. See (#json-ld-context).

 *  `id`: the identifier of this update, it **MUST** be the same value as the subscription update's
    topic

 *  `type`: the fixed value `Subscription`

 *  `topic`: the topic selector used of this subscription

 *  `subscriber`: the topic identifier of the subscriber. It **SHOULD** be an IRI.

 *  `active`: `true` when the subscription is active, and `false` when it is terminated

 *  `payload` (optional): the content of `mercure.payload` in the subscriber's JWS (see
    (#authorization))

The JSON-LD document **MAY** contain other properties.

In order to only allow authorized subscribers to receive subscription events, the subscription
update **MUST** be marked as `private`.

Example:

~~~ json
{
   "id": "/.well-known/mercure/subscriptions/https%3A%2F%2Fexample.com%2F%7Bselector%7D/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6",
   "type": "Subscription",
   "topic": "https://example.com/{selector}",
   "subscriber": "urn:uuid:bb3de268-05b0-4c65-b44e-8f9acefc29d6",
   "active": true,
   "payload": {"foo": "bar"}
}
~~~

## Subscription API

If the hub supports subscription events (see (#subscription-events)), it **SHOULD** also expose
active subscriptions through a web API.

For instance, subscribers interested in maintaining a list of active subscriptions can call the web
API to retrieve them, and then use subscription events (see (#subscription-events)) to keep it up to
date.

The web API **MUST** expose endpoints following these patterns:

 *  `/.well-known/subscriptions`: the collection of subscriptions

 *  `/.well-known/subscriptions/{topic}`: the collection subscriptions for the given topic selector

 *  `/.well-known/subscriptions/{topic}/{subscriber}`: a specific subscription

To access to the URLs exposed by the web API, clients **MUST** be authorized according to the rules
defined in (#authorization). The requested URL **MUST** match at least one of the topic selectors
provided in the `mercure.subscribe` key of the JWS.

The web API **MUST** set the `Content-Type` HTTP header to `application/ld+json`.

URLs returning a single subscription (following the pattern
`/.well-known/subscriptions/{topic}/{subscriber}`) **MUST** expose the same JSON-LD document as
described in (#subscription-events). If the requested subscription does not exist, a `404` status
code **MUST** be returned.

If the requested subscription isn't active anymore, the hub can either return the JSON-LD document
with the `active` property set to `false` or return a `404` status code. Accordingly, collection
endpoints **CAN** return terminated connections with the `active` property set to `false` or omit
them.

Collection endpoints **MUST** return JSON-LD documents containing at least the following properties:

 *  `@context`: the fixed value `https://mercure.rocks/`. `@context` can be omitted if already
    defined in a parent node. See (#json-ld-context).

 *  `id`: the URL used to retrieve the document

 *  `type`: the fixed value `Subscriptions`

 *  `subscriptions`: an array of subscription documents as described in (#subscription-events)

In addition, all endpoints **MUST** set the `lastEventID` property at the root of the returned
JSON-LD document:

 *  `lastEventID`: the identifier of the last event dispatched by the hub at the time of this
    request (see (#reconciliation)). The value **MUST** be `earliest` if no events have been
    dispatched yet. The value of this property **SHOULD** be passed back to the hub when subscribing
    to subscription events to prevent data loss.

As data returned by this web API is volatile, clients **SHOULD** validate that a response coming
from cache is still valid before using it.

Examples:

~~~ http
GET /.well-known/subscriptions HTTP/1.1
Host: example.com

HTTP/1.1 200 OK
Content-type: application/ld+json
Link: <https://example.com/.well-known/mercure>; rel="mercure"
ETag: urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb
Cache-control: must-revalidate

{
   "@context": "https://mercure.rocks/",
   "id": "/.well-known/subscriptions",
   "type": "Subscriptions",
   "lastEventID": "urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb",
   "subscriptions": [
      {
         "id": "/.well-known/mercure/subscriptions/https%3A%2F%2Fexample.com%2F%7Bselector%7D/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6",
         "type": "Subscription",
         "topic": "https://example.com/{selector}",
         "subscriber": "urn:uuid:bb3de268-05b0-4c65-b44e-8f9acefc29d6",
         "active": true,
         "payload": {"foo": "bar"}
      },
      {
         "id": "/.well-known/mercure/subscriptions/https%3A%2F%2Fexample.com%2Fa-topic/urn%3Auuid%3A1e0cba4c-4bcd-44f0-ae8a-7b76f7ef1280",
         "type": "Subscription",
         "topic": "https://example.com/a-topic",
         "subscriber": "urn:uuid:1e0cba4c-4bcd-44f0-ae8a-7b76f7ef1280",
         "active": true,
         "payload": {"baz": "bat"}
      },
      {
         "id": "/.well-known/mercure/subscriptions/https%3A%2F%2Fexample.com%2F%7Bselector%7D/urn%3Auuid%3Aa6c49794-5f74-4723-999c-3a7e33e51d49",
         "type": "Subscription",
         "topic": "https://example.com/{selector}",
         "subscriber": "urn:uuid:a6c49794-5f74-4723-999c-3a7e33e51d49",
         "active": true,
         "payload": {"foo": "bap"}
      }
   ]
}
~~~

~~~ http
GET /.well-known/subscriptions/https%3A%2F%2Fexample.com%2F%7Bselector%7D HTTP/1.1
Host: example.com

HTTP/1.1 200 OK
Content-type: application/ld+json
Link: <https://example.com/.well-known/mercure>; rel="mercure"
ETag: urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb
Cache-control: must-revalidate

{
   "@context": "https://mercure.rocks/",
   "id": "/.well-known/subscriptions/https%3A%2F%2Fexample.com%2F%7Bselector%7D",
   "type": "Subscriptions",
   "lastEventID": "urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb",
   "subscriptions": [
      {
         "id": "/.well-known/mercure/subscriptions/https%3A%2F%2Fexample.com%2F%7Bselector%7D/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6",
         "type": "Subscription",
         "topic": "https://example.com/{selector}",
         "subscriber": "urn:uuid:bb3de268-05b0-4c65-b44e-8f9acefc29d6",
         "active": true,
         "payload": {"foo": "bar"}
      },
      {
         "id": "/.well-known/mercure/subscriptions/https%3A%2F%2Fexample.com%2F%7Bselector%7D/urn%3Auuid%3Aa6c49794-5f74-4723-999c-3a7e33e51d49",
         "type": "Subscription",
         "topic": "https://example.com/{selector}",
         "subscriber": "urn:uuid:a6c49794-5f74-4723-999c-3a7e33e51d49",
         "active": true,
         "payload": {"foo": "bap"}
      }
   ]
}
~~~

~~~ http
GET /.well-known/mercure/subscriptions/https%3A%2F%2Fexample.com%2F%7Bselector%7D/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6 HTTP/1.1
Host: example.com

HTTP/1.1 200 OK
Content-type: application/ld+json
Link: <https://example.com/.well-known/mercure>; rel="mercure"
ETag: urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb
Cache-control: must-revalidate

{
   "@context": "https://mercure.rocks/",
   "id": "/.well-known/mercure/subscriptions/https%3A%2F%2Fexample.com%2F%7Bselector%7D/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6",
   "type": "Subscription",
   "topic": "https://example.com/{selector}",
   "subscriber": "urn:uuid:bb3de268-05b0-4c65-b44e-8f9acefc29d6",
   "active": true,
   "payload": {"foo": "bar"},
   "lastEventID": "urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb"
}
~~~

# JSON-LD Context

The JSON-LD context available at `https://mercure.rocks` is the following:

~~~ json
{
"@context": {
   "@vocab": "_:",
   "mercure": "https://mercure.rocks/",
   "id": "@id",
   "type": "@type",
   "Subscription": "mercure:Subscription",
   "Subscriptions": "mercure:Subscriptions",
   "subscriptions": "mercure:subscriptions",
   "topic": "mercure:topic",
   "subscriber": "mercure:subscriber",
   "active": "mercure:active",
   "payload": "mercure:payload",
   "lastEventID": "mercure:lastEventID"
}
~~~

# Encryption

Using HTTPS does not prevent the hub from accessing the update's content. Depending of the intended
privacy of information contained in the update, it **MAY** be necessary to prevent eavesdropping by
the hub.

To make sure that the message content can not be read by the hub, the publisher **MAY** encrypt the
message before sending it to the hub. The publisher **SHOULD** use JSON Web Encryption [@!RFC7516]
to encrypt the update content. The publisher **MAY** provide the URL pointing to the relevant
encryption key(s) in the `key-set` attribute of the Link HTTP header during the discovery. See
(#discovery). The `key-set` attribute **MUST** contain a key encoded using the JSON Web Key Set
[@!RFC7517] format. Any other out-of-band mechanism **MAY** be used instead to share the key between
the publisher and the subscriber.

Update encryption is considered a best practice to prevent mass surveillance. This is especially
relevant if the hub is managed by an external provider.

# IANA Considerations

## Well-Known URIs Registry

A new "well-known" URI as described in (#discovery) has been registered in the "Well-Known URIs"
registry as described below:

 *  URI Suffix: mercure

 *  Change Controller: IETF

 *  Specification document(s): This specification, (#discovery)

 *  Related information: N/A

## Link Relation Types Registry

A new "Link Relation Type" as described in (#discovery) has been registered in the "Link Relation
Type" registry with the following entry:

 *  Relation Name: mercure

 *  Description: The Mercure Hub to use to subscribe to updates of this resource.

 *  Reference: This specification, (#discovery)

# Security Considerations

The confidentiality of the secret key(s) used to generate the JWSs is a primary concern. The
secret key(s) **MUST** be stored securely. They **MUST** be revoked immediately in the event of
compromission.

Possessing valid JWSs allows any client to subscribe, or to publish to the hub. Their
confidentiality **MUST** therefore be ensured. To do so, JWSs **MUST** only be transmitted over
secure connections.

Also, when the client is a web browser, the JWS **SHOULD** not be made accessible
to JavaScript scripts for resilience against [Cross-site Scripting (XSS)
attacks](https://owasp.org/www-community/attacks/xss/). It's the main reason why, when the client
is a web browser, using `HttpOnly` cookies as the authorization mechanism **SHOULD** always be
preferred.

In the event of compromission, revoking JWSs before their expiration is often difficult. To that
end, using short-lived tokens is strongly **RECOMMENDED**.

The publish endpoint of the hub may be targeted by [Cross-Site Request Forgery (CSRF)
attacks](https://owasp.org/www-community/attacks/csrf) when the cookie-based authorization mechanism
is used. Therefore, implementations supporting this mechanism **MUST** mitigate such attacks.

The first prevention method to implement is to set the `mercureAuthorization`
cookie's `SameSite` attribute. However, [some web browsers still not support this
attribute](https://caniuse.com/#feat=same-site-cookie-attribute) and will remain vulnerable.
Additionally, hub implementations **SHOULD** use the `Origin` and `Referer` HTTP headers set by web
browsers to verify that the source origin matches the target origin. If none of these headers are
available, the hub **SHOULD** discard the request.

CSRF prevention techniques, including those previously mentioned, are described
in depth in [OWASP's Cross-Site Request Forgery (CSRF) Prevention Cheat
Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html).

# Implementation Status

[RFC Editor Note: Please remove this entire seciton prior to publication as an RFC.]

This section records the status of known implementations of the protocol defined by this
specification at the time of posting of this Internet-Draft, and is based on a proposal described
in [@RFC6982]. The description of implementations in this section is intended to assist the IETF in
its decision processes in progressing drafts to RFCs. Please note that the listing of any individual
implementation here does not imply endorsement by the IETF. Furthermore, no effort has been spent to
verify the information presented here that was supplied by IETF contributors. This is not intended
as, and must not be construed to be, a catalog of available implementations or their features.
Readers are advised to note that other implementations may exist. According to RFC 6982, "this will
allow reviewers and working groups to assign due consideration to documents that have the benefit
of running code, which may serve as evidence of valuable experimentation and feedback that have
made the implemented protocols more mature. It is up to the individual working groups to use this
information as they see fit."

## Mercure.rocks Hub

Organization responsible for the implementation:

Dunglas Services SAS Les-Tilleuls.coop

Implementation Name and Details:

Mercure.rocks, available at <https://mercure.rocks>

Brief Description:

This is the reference implementation of the Mercure hub. It is written in Go and is optimized for
performance.

Level of Maturity:

Widely used.

Coverage:

All the features of the protocol.

Version compatibility:

The implementation follows the latest draft.

Licensing:

All code is covered under the GNU Affero Public License version 3 or later.

Implementation Experience:

Used in production.

Contact Information:

Kévin Dunglas, [dunglas+mercure@gmail.com](mailto:dunglas+mercure@gmail.com)
<https://mercure.rocks>

Interoperability:

Reported compatible with all major browsers and server-side tools.

## Ilshidur/node-mercure

Implementation Name and Details:

Ilshidur/node-mercure, <https://github.com/Ilshidur/node-mercure>

Brief Description:

Hub and Publisher implemented in Node.

Level of Maturity:

Beta, not suitable for production.

Coverage:

All the features of the protocol except the subscription events.

Version compatibility:

The implementation currently follows the revision 5 of the draft.

Licensing:

All code is covered under the GNU Public License version 3 or later.

Contact Information:

<https://github.com/Ilshidur/node-mercure>

Interoperability:

Reported compatible with all major browsers and server-side tools.

## Symfony

Implementation Name and Details:

Symfony Mercure Component, available at <https://symfony.com/doc/current/components/mercure.html>

Brief Description:

This a publisher library written in PHP. It also provides support for Mercure in the Symfony web
framework.

Level of Maturity:

Widely used.

Coverage:

All the publisher features of the protocol.

Version compatibility:

The implementation follows the latest draft.

Licensing:

All code is covered under the MIT license.

Implementation Experience:

Used in production.

Contact Information:

<https://symfony.com>

Interoperability:

Reported compatible with the Mercure.rocks Hub.

## API Platform

Implementation Name and Details:

API Platform, available at <https://api-platform.com/docs/core/mercure/>

Brief Description:

The API Platform framework, allows to create async APIs implementing the Mercure protocol and to
generate clients for these APIs.

Level of Maturity:

Widely used.

Coverage:

All the publisher and consumer features of the protocol.

Version compatibility:

The implementation follows the latest draft.

Licensing:

All code is covered under the MIT license.

Implementation Experience:

Used in production.

Contact Information:

<https://api-platform.com>

Interoperability:

Reported compatible with the reference implementation of the Mercure Hub.

## Laravel Mercure Broadcaster

Implementation Name and Details:

Laravel Mercure Broadcaster, available at
<https://github.com/mvanduijker/laravel-mercure-broadcaster>

Brief Description:

Laravel broadcaster for Mercure. Use the Mercure protocol as transport for Laravel Broadcast.

Level of Maturity:

Production

Coverage:

All the publisher features of the protocol.

Version compatibility:

The implementation currently follows the revision 5 of the draft. An open Pull Request adds support
for the latest version of the draft.

Licensing:

All code is covered under the MIT license.

Implementation Experience:

Used in production.

Contact Information:

<https://github.com/mvanduijker/laravel-mercure-broadcaster>

Interoperability:

Reported compatible with the reference implementation of the Mercure Hub.

## dart_mercure

Implementation Name and Details:

dart*mercure, available at <https://github.com/wallforfry/dart>*mercure

Brief Description:

Publisher and Subscriber library for Dart / Flutter.

Level of Maturity:

Stable

Coverage:

All the publisher and subscriber features of the protocol.

Version compatibility:

The implementation follows the latest draft.

Licensing:

All code is covered under the BSD 2-Clause "Simplified" License.

Contact Information:

<https://github.com/wallforfry/dart_mercure>

Interoperability:

Reported compatible with the reference implementation of the Mercure Hub.

## pymercure

Implementation Name and Details:

pymercure, available at <https://github.com/vitorluis/python-mercure>

Brief Description:

Publisher and Subscriber library for Python.

Level of Maturity:

Alpha

Coverage:

All the publisher and subscriber features of the protocol.

Version compatibility:

The implementation currently follows the revision 5 of the draft. An open Pull Request adds support
for the latest version of the draft.

Licensing:

All code is covered under the BSD 2-Clause "Simplified" License.

Contact Information:

<https://github.com/vitorluis/python-mercure>

Interoperability:

Reported compatible with the reference implementation of the Mercure Hub.

## Amphp Mercure Publisher

Implementation Name and Details:

Amphp Mercure Publisher, available at <https://github.com/eislambey/amp-mercure-publisher>

Brief Description:

Async Mercure publisher based on Amphp.

Level of Maturity:

Stable

Coverage:

All the publisher features of the protocol.

Version compatibility:

The implementation currently follows the revision 5 of the draft. An open Pull Request adds support
for the latest version of the draft.

Licensing:

All code is covered under the MIT license.

Contact Information:

<https://github.com/eislambey/amp-mercure-publisher>

Interoperability:

Reported compatible with the reference implementation of the Mercure Hub.

## Java Library for Mercure

Implementation Name and Details:

Java Library for Mercure, available at <https://github.com/vitorluis/java-mercure>

Brief Description:

Java library to publish messages to a Mercure Hub!

Level of Maturity:

Alpha

Coverage:

All the publisher features of the protocol.

Version compatibility:

The implementation currently follows the revision 5 of the draft. An open Pull Request adds support
for the latest version of the draft.

Licensing:

All code is covered under the MIT license.

Contact Information:

<https://github.com/vitorluis/java-mercure>

Interoperability:

Reported compatible with the reference implementation of the Mercure Hub.

## Yii 2 Mercure behavior

Implementation Name and Details:

Yii 2 Mercure behavior, available at <https://github.com/bizley/mercure-behavior>

Brief Description:

Yii 2 behavior to automatically publish updates to a Mercure hub.

Level of Maturity:

Stable

Coverage:

All the publisher features of the protocol.

Version compatibility:

The implementation currently follows the revision 5 of the draft.

Licensing:

All code is covered under the Apache License 2.0.

Contact Information:

<https://github.com/bizley/mercure-behavior>

Interoperability:

Reported compatible with the reference implementation of the Mercure Hub.

## GitHub Action for Mercure

Implementation Name and Details:

GitHub Action for Mercure, available at
<https://github.com/marketplace/actions/github-action-for-mercure>

Brief Description:

Send a Mercure update when a GitHub event occurs.

Level of Maturity:

Stable

Coverage:

All the publisher features of the protocol.

Version compatibility:

The implementation currently follows the latest version of the draft.

Licensing:

All code is covered under the GNU Public License version 3 or later.

Contact Information:

<https://github.com/Ilshidur/action-mercure>

Interoperability:

Reported compatible with the reference implementation of the Mercure Hub.

## Other Implementations

Other implementations can be found on GitHub: <https://github.com/topics/mercure>

# Acknowledgements

Parts of this specification, especially (#discovery) have been adapted from the WebSub
recommendation [@W3C.REC-websub-20180123]. The editor wish to thanks all the authors of this
specification.

{backmatter}
