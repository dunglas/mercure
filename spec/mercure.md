%%%
title = "The Mercure Protocol"
area = "Applications and Real-Time"
workgroup = "HTTP"
submissiontype = "IETF"

[seriesInfo]
name = "Internet-Draft"
value = "draft-dunglas-mercure-07"
stream = "IETF"
status = "standard"

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

Mercure provides a common publish-subscribe mechanism for public and private web resources.
It pushes any web content to web browsers and other clients in a fast, reliable, and
battery-efficient way. Mercure is especially useful for delivering real-time updates of
resources served through sites and web APIs to web and mobile applications, and can also
be used as a general-purpose publish-subscribe system.

Subscription requests are relayed through hubs, which validate them.
When new or updated content becomes available, hubs check whether subscribers are authorized
to receive it and then distribute it.

{mainmatter}

# Terminology

The keywords **MUST**, **MUST NOT**, **REQUIRED**, **SHALL**, **SHALL NOT**, **SHOULD**, **SHOULD
NOT**, **RECOMMENDED**, **MAY**, and **OPTIONAL**, when they appear in this document, are to be
interpreted as described in [@!RFC2119].

*   Topic: The unit to which one can subscribe for changes. The topic is identified by a string
    that **MAY** be an IRI [@!RFC3987].
*   Update: The message containing the updated version of the topic. An update can be marked as
    private; in that case, it **MUST** be dispatched only to subscribers allowed to receive it.
*   Topic matcher: An expression matched against one or more topics,
    depending on the matcher type.
*   Topic matcher type: The type of a matching expression,
    such as exact match, regular expression, or URL pattern.
*   Publisher: An owner of a topic. Notifies the hub when the topic feed has been updated. As in
    almost all pub-sub systems, the publisher is unaware of the subscribers, if any. Other pub-sub
    systems might call the publisher the "source". Typically a site or a web API, but it can also
    be a web browser.
*   Subscriber: A client application that subscribes to real-time updates of topics using topic
    matchers. Typically a web or a mobile application, but it can also be a server.
*   Subscription: A topic matcher used by a subscriber to receive updates. A single subscriber can
    have several subscriptions by providing several topic matchers.
*   Hub: A server that handles subscription requests and distributes content to subscribers when
    the corresponding topics have been updated. A hub **MAY** implement its own policies on who
    can use it.

# Subscription

The subscriber subscribes to a URL exposed by a hub to receive updates from one or more topics.
To subscribe, the client opens an HTTPS connection to the hub's subscription URL (advertised by
the publisher) following the Server-Sent Events specification [@!W3C.REC-eventsource-20150203].
The `GET` HTTP method **MUST** be used. The connection **SHOULD** use HTTP version 2 or higher
to leverage multiplexing and other performance-related features.

The subscriber specifies the list of topics to receive updates from using one or more query
parameters named `match`. The subscriber receives updates only for topics matching exactly one
of the `match` query parameters.

The subscriber can also use other matcher types via query parameters whose name is `match`
concatenated with the matcher type name (e.g., `matchRegexp`, `matchURLPattern`).

The `matchExact` query parameter **MUST** be treated as equivalent to `match`.

The hub **MUST** ignore the case of query parameters starting with `match`.
For instance, `matchExact`, `matchEXACT`, `matchexact`, and `MaTCheXaCt` **MUST** be considered
the same query parameter.

If the type of one or more matchers is not supported by the hub, it **MUST** respond with a
501 "Not Implemented" HTTP status code.

The subscriber receives updates for all topics matching at least one topic matcher according to
the matcher type rules.

The protocol does not specify the maximum number of query parameters that can be sent, but the
hub **MAY** apply an arbitrary limit. The hub **MAY** also enforce an implementation-defined
maximum length for the pattern of each topic matcher. Requests exceeding any such limit **MUST**
be rejected with a 400 "Bad Request" HTTP status code. A subscription is created for every
provided parameter starting with the string `match`. See (#subscription-events).

The `EventSource` JavaScript interface [@!eventsource-interface] **MAY** be used to establish
the connection. Any other appropriate mechanism, including but not limited to readable streams
[@W3C.NOTE-streams-api-20161129] and XMLHttpRequest [@!xhr] (used by popular polyfills),
**MAY** also be used.

The hub sends updates to the subscriber for topics matching the provided topic matchers.

If an update is marked as `private`, the hub **MUST NOT** dispatch it to subscribers not authorized
to receive it. See (#authorization).

The hub **MUST** send these updates as `text/event-stream`-compliant events
[@!W3C.REC-eventsource-20150203].

The `data` property **MUST** contain the topic's new version. It **MAY** be the full resource or
a partial update in formats such as JSON Patch [@RFC6902] or JSON Merge Patch [@RFC7386].

All other properties defined in the Server-Sent Events specification **MAY** be used and **MUST**
be supported by hubs.

The resource **MAY** be represented in a format with hypermedia capabilities such as
JSON-LD [@W3C.REC-json-ld-20140116], Atom [@RFC4287], XML [@W3C.REC-xml-20081126] or HTML
[@W3C.REC-html52-20171214].

Web Linking [@!RFC5988] **MAY** be used to indicate the IRI of the resource sent in the event.
When using Atom, XML, or HTML as the serialization format, the document **SHOULD** contain a
`link` element with a `self` relation that holds the IRI of the resource. When using JSON-LD,
the document **SHOULD** contain an `@id` property holding the IRI of the resource.

Example:

~~~ javascript
// The subscriber subscribes to updates
// for the https://example.com/foo topic, the bar topic,
// and to any topic matching the https://example.com/bar/:id URL Pattern
const url = new URL('https://example.com/.well-known/mercure');
url.searchParams.append('match', 'https://example.com/foo');
url.searchParams.append('match', 'bar');
url.searchParams.append('matchURLPattern', 'https://example.com/bar/:id');

const eventSource = new EventSource(url);

// The callback will be called every time an update is published
eventSource.onmessage = function ({data}) {
    console.log(data);
};
~~~

The hub **MAY** require subscribers and publishers to be authenticated, and **MAY** apply extra
authorization rules not defined in this specification.

# Matcher Types

## Exact Matching

The hub **MUST** support exact matching. With this matcher type, the hub **MUST** perform an
exact, case-sensitive comparison between the topic and the matcher.

The matcher type name is `Exact`.
The corresponding query parameters are `match` and `matchExact`.

## URL Pattern

The hub **SHOULD** support using URL patterns [@!urlpattern] as matchers.
URL patterns **SHOULD** be preferred to regular expressions when matching URLs.

URL patterns **MUST** be absolute (e.g., `https://example.com/books/:id`).
Because topics are absolute IRIs [@!RFC3987] and the protocol defines no base URL,
relative patterns have no portable resolution. The hub **MUST** reject them.

The matcher type name is `URLPattern`.
The corresponding query parameter is `matchURLPattern`.

## Regular Expression

The hub **SHOULD** support using I-Regexp regular expressions [@!RFC9485] as matchers.

The matcher type name is `Regexp`.
The corresponding query parameter is `matchRegexp`.

## Common Expression Language (CEL)

The hub **MAY** support using CEL expressions [@cel] as matchers.

A variable named `topics` containing an array of strings **MUST** be passed to the expression.
This variable **MUST** contain the canonical topic followed by the alternate topics of the
update to match.

The hub **MAY** also pass implementation-specific variables and expose implementation-specific
functions.

The expression **MUST** return a boolean value: `true` if the topic matches, `false` otherwise.

If parsing or checking of a CEL expression fails, or if the expression does not return a boolean
value, the hub **MUST** return a 400 "Bad Request" HTTP status code.

To mitigate denial-of-service attacks by clients submitting pathological expressions,
hubs implementing CEL **SHOULD** enforce an implementation-defined evaluation cost limit.
When the limit is reached during evaluation, the expression **MUST** be treated as returning
`false` and the evaluation **MUST** be aborted. Hubs **MAY** additionally log the event.

The matcher type name is `CEL`.
The corresponding query parameter is `matchCEL`.

## URI Template

The hub **MAY** support using URI templates [@!RFC6570] as matchers.
Whenever possible, URL patterns **SHOULD** be preferred.

The matcher type name is `URITemplate`.
The corresponding query parameter is `matchURITemplate`.

## Summary of Matcher Types

| Matcher Type   | Query Parameter      | Requirement  |
|----------------|----------------------|--------------|
| `Exact`        | `match` / `matchExact` | **MUST**   |
| `URLPattern`   | `matchURLPattern`    | **SHOULD**   |
| `Regexp`       | `matchRegexp`        | **SHOULD**   |
| `CEL`          | `matchCEL`           | **MAY**      |
| `URITemplate`  | `matchURITemplate`   | **MAY**      |

## Other Matcher Types

The hub **MAY** implement additional matcher types, including implementation-specific ones.

# Publication

The publisher sends updates by issuing `POST` HTTPS requests to the hub URL. When it receives an
update, the hub dispatches it to subscribers using the established server-sent events connections.

The hub **MAY** also dispatch the update using other protocols such as WebSub
[@W3C.REC-websub-20180123] or ActivityPub [@W3C.REC-activitypub-20180123].

An application **MAY** deliver events directly to subscribers without an external hub. In that
case, the publish endpoint described in this section is not required.

The request **MUST** be encoded using the `application/x-www-form-urlencoded` format
[@W3C.REC-html52-20171214] and **MUST** contain at least one `topic` field. It **MAY** also contain
the following name-value tuples:

*   `topic`: The identifiers of the updated topic. It is **RECOMMENDED** to use an IRI as
    identifier. If this name is present several times, the first occurrence is the canonical IRI
    of the topic and the remaining ones are alternate IRIs. The hub **MUST** dispatch the update
    to subscribers that are subscribed to either the canonical IRI or any of its alternate IRIs.
*   `data` (optional): the content of the new version of this topic.
*   `private` (optional): if this name is set, the update **MUST NOT** be dispatched to subscribers
    not authorized to receive it. See (#authorization). It is **RECOMMENDED** to set the value to
    `on`, but it **MAY** contain any value, including an empty string.
*   `id` (optional): the topic's revision identifier; used as the SSE `id` property.
    The provided ID **MUST NOT** start with the `#` character. The provided ID **MAY** be a valid
    IRI. If omitted, the hub **MUST** generate a valid IRI [@!RFC3987]. A UUID [@RFC4122] or a
    [DID](https://www.w3.org/TR/did-core/) **MAY** be used. Alternatively, the hub **MAY** generate
    a relative URI composed of a fragment (starting with `#`). This is convenient to return an
    offset or a sequence that is unique for this hub. The hub **MAY** ignore the client-supplied
    ID and generate its own.
*   `type` (optional): the SSE `event` property (a specific event type).
*   `retry` (optional): the SSE `retry` property (the reconnection time).

On success, the hub **MUST** return a successful HTTP status code, and the response body **MUST**
be the `id` generated by the hub for the update. The publisher **MUST** be authorized to publish
updates; see (#authorization).

Example:

~~~ http
POST /.well-known/mercure
Host: example.com
Content-Type: application/x-www-form-urlencoded
Authorization: Bearer [snip]

topic=https://example.com/foo&data=the%20content

200 OK
Content-type: text/plain

urn:uuid:e1ee88e2-532a-4d6f-ba70-f0f8bd584022
~~~

# Authorization

To prove that they are authorized, both publishers and subscribers **MUST** present a valid JWS
[@!RFC7515] in compact serialization to the hub. This JWS **SHOULD** be short-lived, especially
when the subscriber is a web browser. A different key **MAY** be used to sign subscribers' and
publishers' tokens.

Three mechanisms are defined to present the JWS to the hub:

*   an `Authorization` HTTP header,
*   a cookie,
*   an `authorization` URI query parameter.

When any authorization mechanism is used, the connection **MUST** use an encryption layer such
as HTTPS.

If the client presents an `Authorization` HTTP header, the JWS it contains **MUST** be used.
The content of the `authorization` query parameter and of the cookie **MUST** be ignored.

If the client sets an `authorization` query parameter and presents no `Authorization` HTTP header,
the content of the query parameter **MUST** be used, and the content of the cookie **MUST** be
ignored.

If the client attempts an operation it is not authorized to perform, the hub **SHOULD** return a
403 HTTP status code.

## Authorization HTTP Header

If the publisher or the subscriber is not a web browser, it **SHOULD** use an `Authorization`
HTTP header. This header **MUST** contain the string `Bearer` followed by a space character and
by the JWS. The hub checks that the JWS conforms to the rules (defined later) ensuring that the
client is authorized to publish or subscribe to updates.

## Cookie

Per the `EventSource` specification [@W3C.REC-eventsource-20150203], web browsers cannot set
custom HTTP headers on such connections, and the connections can only be established using the
`GET` HTTP method. However, cookies are supported and can be included even in cross-domain
requests if [the CORS credentials are
set](https://html.spec.whatwg.org/multipage/server-sent-events.html#dom-eventsourceinit-withcredentials).

If the publisher or the subscriber is a web browser, it **SHOULD**, whenever possible, send a
cookie containing the JWS when connecting to the hub. It is **RECOMMENDED** to name the cookie
`mercureAuthorization`, but a different name **MAY** be used to prevent conflicts when several
hubs share the same domain.

The cookie **SHOULD** be set during discovery (see (#discovery)) to improve overall security.
Consequently, if the cookie is set during discovery, the publisher and the hub must share the
same second-level domain. The `Domain` attribute **MAY** be used to allow the publisher and the
hub to use different subdomains. See (#discovery).

The cookie **SHOULD** have the `Secure`, `HttpOnly`, and `SameSite` attributes set. The cookie's
`Path` attribute **SHOULD** also be set to the hub's URL. See (#security-considerations).

## URI Query Parameter

If the client cannot use an `Authorization` HTTP header or a cookie, the JWS **MAY** be passed
as a URI query component as defined by "Uniform Resource Identifier (URI): Generic Syntax"
[@!RFC3986], using the `authorization` parameter.

The `authorization` query parameter **MUST** be separated from topic matcher parameters and from
other request-specific parameters using `&` characters (ASCII code 38).

For example, the client makes the following HTTP request using transport-layer security:

~~~ http
GET /.well-known/mercure?match=https://example.com/books/foo&authorization=<JWS>
Host: hub.example.com
~~~

Clients using the URI Query Parameter method **SHOULD** also send a `Cache-Control` header
containing the `no-store` option. Server success (2XX status) responses to these requests
**SHOULD** contain a `Cache-Control` header with the `private` option.

Because of the security weaknesses associated with the URI method (see
(#security-considerations)), including the high likelihood that the URL containing the access
token will be logged, this method **SHOULD NOT** be used unless it is impossible to transport
the access token in the `Authorization` request header field or a secure cookie. Hubs **MAY**
support this method.

## Publishers

Publishers **MUST** be authorized by the hub and **MUST** prove they are authorized to publish
updates for every topic of the update.

To be allowed to publish an update, the JWS presented by the publisher **MUST** contain a claim
named `mercure`, and this claim **MUST** contain a `publish` key. `mercure.publish` contains an
array of topic matchers as defined in (#topic-matcher-list).

If `mercure.publish` is not defined or contains an empty array, the publisher **MUST NOT** be
authorized to dispatch any update. Otherwise, the hub **MUST** check that every topic of the
update to dispatch matches at least one of the topic matchers contained in `mercure.publish`.

If the publisher is not authorized for every topic of an update, the hub **MUST NOT** dispatch
the update (even if some topics are allowed) and **MUST** return a 403 HTTP status code.

## Subscribers

To receive updates marked as `private`, a subscriber **MUST** prove that it is authorized for at
least one topic of the update. If the subscriber is not authorized, it **MUST NOT** receive the
update.

If the presented JWS contains an expiration time in the standard `exp` claim defined in
[@!RFC7519], the hub **MUST** close the connection at that time.

To receive updates marked as `private`, the JWS presented by the subscriber **MUST** have a claim
named `mercure` containing a `subscribe` key. `mercure.subscribe` contains an array of topic
matchers as described in (#topic-matcher-list).

The hub **MUST** check that at least one topic of the update to dispatch (*canonical* or
*alternate*) matches at least one topic matcher provided in `mercure.subscribe`.

This behavior allows subscribing to several topics using topic matchers while guaranteeing that
only authorized subscribers receive private updates (even if their canonical topics are matched
by these matchers).

For example, a subscriber wants to receive updates concerning all *book* resources it has access
to. It can use the URL Pattern `https://example.com/books/:id` as the value of the
`matchURLPattern` query parameter. Adding this same URL Pattern to the `mercure.subscribe` claim
would allow the subscriber to receive all updates for all book resources, which is not the
desired behavior: this subscriber is only authorized to access **some** of those resources.

To solve this problem, the `mercure.subscribe` claim can contain a URL Pattern topic matcher such
as `https://example.com/users/foo/?topic=:topic`.

The publisher then publishes a private update with `https://example.com/books/1` as the canonical
topic and `https://example.com/users/foo/?topic=https%3A%2F%2Fexample.com%2Fbooks%2F1` as an
alternate topic:

~~~ http
POST /.well-known/mercure
Host: example.com
Content-Type: application/x-www-form-urlencoded
Authorization: Bearer [snip]

topic=https://example.com/books/1&topic=https://example.com/users/foo/?topic=https%3A%2F%2Fexample.com%2Fbooks%2F1&private=on
~~~

The subscriber is subscribed to `https://example.com/books/:id`, a URL Pattern matched by the
canonical topic of the update. The canonical topic is not matched by the topic matcher in
`mercure.subscribe`. However, the alternate topic
`https://example.com/users/foo/?topic=https%3A%2F%2Fexample.com%2Fbooks%2F1` is. Consequently,
this private update is delivered to the subscriber. Other updates whose canonical topic is
matched by `matchURLPattern` but whose alternate topics are not matched by `mercure.subscribe`
are not delivered.

## Topic Matcher List

Topic matchers present in the `mercure.subscribe` or `mercure.publish` claim **MUST** be objects.

Each object **MUST** have a `match` property containing the topic matcher itself, and **MAY**
have an OPTIONAL `matchType` property containing the topic matcher type. The value of the
`matchType` key **MUST** be considered case-insensitive. If no `matchType` key is present, the
hub **MUST** assume the `Exact` matcher type.

For backward compatibility, a matcher **MAY** also be represented as a bare string when the hub
is explicitly operating in a deprecated-protocol compatibility mode (see the hub's
configuration). In that mode, the string **MUST** be interpreted using the version 8 rules
("exact OR URI Template"). Outside of compatibility mode, bare strings **MUST** be rejected with
a 401 "Unauthorized" HTTP status code. Silently reinterpreting them as `Exact` could change the
semantics of tokens minted for earlier protocol versions.

If the type of one or more matchers in `mercure.subscribe` is not supported by the hub, the hub
**MUST** reject the subscription request with a 501 "Not Implemented" HTTP status code and
**MUST NOT** establish the subscription.

If the type of one or more matchers in `mercure.publish` is not supported by the hub, the hub
**MUST** reject the publication request with a 501 "Not Implemented" HTTP status code and
**MUST NOT** dispatch the update.

## Payloads

User-defined data can be attached to subscriptions and made available through the subscription
API and in subscription events. See (#subscription-events).

Each entry in the `mercure.subscribe` claim represented as an object **MAY** contain a JSON
object under the `payload` key.

The `payload` value associated with the first topic matcher in the `mercure.subscribe` claim
that matches the subscription's own matcher (as determined by the `match` and `matchType` query
parameters) **MUST** be included under the `payload` key in the JSON object describing a
subscription, both in the subscription API and in subscription events.

A claim matcher is considered to match a subscription matcher when any of the following holds:

1.  The claim matcher's pattern is the reserved string `*`.
2.  The claim matcher's type is the same as the subscription matcher's type (case-insensitive)
    and both patterns are identical.
3.  The claim matcher, evaluated against the subscription matcher's `match` value as if that
    value were a topic, returns true. For instance, a claim with `matchType=URLPattern` and
    `match=https://example.com/:id` matches a subscription with `matchType=Exact` and
    `match=https://example.com/42`, because the URL pattern accepts that URL.

If no claim matches the subscription, the hub **MUST** fall back to the top-level
`mercure.payload` value, if any.

Example JWT document containing payloads:

~~~ json
{
    "subscribe": [
        {
            "match": "https://example.com/foo",
            "payload": {
                "custom1": "data only available for subscriptions to this topic"
            }
        },
        {
            "match": "https://example.com/bar/:id",
            "matchType": "URLPattern",
            "payload": {
                "custom2": "data only available for subscriptions matching this matcher"
            }
        },
        {
            "match": ".*",
            "matchType": "Regexp",
            "payload": {
                "custom3": "data available for all other subscriptions"
            }
        }
    ]
}
~~~

For instance, a payload can contain the user ID of the subscriber, its username, a list of
groups it belongs to, or its IP address. Storing data in payloads is a convenient way to share
data related to one subscriber with other subscribers.

# Reconnection, State Reconciliation, and Event Sourcing {#reconciliation}

The protocol allows reconciliation of state after a reconnection. It can also be used to
implement an [Event store](https://en.wikipedia.org/wiki/Event_store).

To allow re-establishment in case of connection loss, events dispatched by the hub **MUST**
include an `id` property. The value of this property **SHOULD** be an IRI [@!RFC3987]. A UUID
[@RFC4122] or a [DID](https://www.w3.org/TR/did-core/) **MAY** be used.

Per the server-sent events specification, the subscriber tries to reconnect automatically in
case of connection loss. During reconnection, the subscriber **MUST** send the last received
event ID in a [Last-Event-ID](https://html.spec.whatwg.org/multipage/iana.html#last-event-id)
HTTP header.

To fetch any update dispatched between the initial resource generation by the publisher and the
connection to the hub, the subscriber **MUST** send the event ID provided during discovery
either as a `Last-Event-ID` header or as a `lastEventID` query parameter. See (#discovery).

`EventSource` implementations may not allow setting HTTP headers on the first connection (before
a reconnection), and web browser implementations do not allow it.

To work around this, the hub **MUST** also accept the last event ID in a query parameter named
`lastEventID`.

If both the `Last-Event-ID` HTTP header and the `lastEventID` query parameter are present, the
HTTP header **MUST** take precedence.

If the `Last-Event-ID` HTTP header or the `lastEventID` query parameter is present, the hub
**SHOULD** send all events published after the one bearing this identifier to the subscriber.

The reserved value `earliest` requests that the hub send all updates it has for the subscribed
topics. The hub **MAY** ignore this request according to its own policy.

The hub **MAY** discard some events for operational reasons. When the request contains a
`Last-Event-ID` HTTP header or a `lastEventID` query parameter, the hub **MUST** set a
`Last-Event-ID` header on the HTTP response. The value of this response header **MUST** be the
ID of the event preceding the first one sent to the subscriber, or the reserved value
`earliest` if there is no preceding event (for example, when the hub history is empty, or when
the requested event does not exist).

The subscriber **SHOULD NOT** assume that no events will be lost (events may be lost, for
instance, if the hub stores only a limited number of events in its history). In some cases (for
example, when sending partial updates in the JSON Patch [@RFC6902] format, or when using the
hub as an event store), lost updates can cause data loss.

To detect data loss, the subscriber **MAY** compare the value of the `Last-Event-ID` response
HTTP header with the last event ID it requested. In case of data loss, the subscriber **SHOULD**
re-fetch the original topic.

Note: Native `EventSource` implementations do not expose HTTP response headers. However,
polyfills and server-sent events clients in most programming languages do.

The hub **MAY** also specify the reconnection time using the `retry` key, as defined by the
server-sent events format.

# Active Subscriptions

Mercure provides a mechanism to track active subscriptions. If the hub supports this optional set
of features, updates will be published when a subscription is created, or terminated, and a web API
exposes the list of active subscriptions.

Variables are templated and expanded following [@!RFC6570].

## Subscription Events

If the hub supports the active subscriptions feature, it **MUST** publish an update every time a
subscription is created or terminated.

The topic of these updates **MUST** be an expansion of
`/.well-known/mercure/subscriptions/{matchType}/{match}/{subscriber}` with the following variables:

*   `{matchType}`: the topic matcher type used for this subscription
*   `{match}`: the topic matcher used for this subscription
*   `{subscriber}`: a unique identifier for the subscriber

Note: Because strings containing reserved characters (e.g., URIs, URL Patterns, and URI
Templates) can be used for the `{match}` and `{subscriber}` variables, per [@!RFC6570] the
values of all variables **MUST** be percent-encoded during expansion.

If a subscriber has several subscriptions, it **SHOULD** be identified by a
`{subscriber}` variable having the same value.

`{subscriber}` **SHOULD** be an IRI [@!RFC3987]. A UUID [@RFC4122] or a
[DID](https://www.w3.org/TR/did-core/) **MAY** also be used.

The content of the update **MUST** be a JSON-LD [@!W3C.REC-json-ld-20140116] document containing
at least the following properties:

*   `@context`: the fixed value `https://mercure.rocks/`. `@context` **MAY** be omitted if
    already defined in a parent node. See (#json-ld-context).
*   `id`: the identifier of this update; **MUST** be the same value as the subscription update's
    topic.
*   `type`: the fixed value `Subscription`.
*   `matchType`: the topic matcher type used for this subscription. The value **MUST** be
    considered case-insensitive.
*   `match`: the topic matcher used for this subscription.
*   `subscriber`: the identifier of the subscriber. It **SHOULD** be an IRI.
*   `active`: `true` when the subscription is active, `false` when it is terminated.
*   `payload` (optional): the content of the `payload` field associated with this subscription
    in the subscriber's JWS (see (#payloads)).

The JSON-LD document **MAY** contain other properties.

To restrict subscription events to authorized subscribers, the subscription update **MUST** be
marked as `private`.

Example:

~~~ json
{
   "id": "/.well-known/mercure/subscriptions/URLPattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6",
   "type": "Subscription",
   "matchType": "URLPattern",
   "match": "https://example.com/:selector",
   "subscriber": "urn:uuid:bb3de268-05b0-4c65-b44e-8f9acefc29d6",
   "active": true,
   "payload": {"foo": "bar"}
}
~~~

## Subscription API

If the hub supports subscription events (see (#subscription-events)), it **SHOULD** also expose
active subscriptions through a web API.

For instance, subscribers interested in maintaining a list of active subscriptions can call the
web API to retrieve them, then use subscription events (see (#subscription-events)) to keep the
list up to date.

The web API **MUST** expose endpoints following these patterns:

*   `/.well-known/mercure/subscriptions`: the collection of subscriptions.
*   `/.well-known/mercure/subscriptions/{matchType}/{match}`: the collection of subscriptions
    for the given topic matcher.
*   `/.well-known/mercure/subscriptions/{matchType}/{match}/{subscriber}`: a specific
    subscription.

To access these URLs, clients **MUST** be authorized according to the rules defined in
(#authorization). The requested URL **MUST** match at least one of the topic matchers provided
in the `mercure.subscribe` key of the JWS.

The web API **MUST** set the `Content-Type` HTTP header to `application/ld+json`.

URLs returning a single subscription (following the pattern
`/.well-known/mercure/subscriptions/{matchType}/{match}/{subscriber}`) **MUST** expose the same
JSON-LD document as described in (#subscription-events). If the requested subscription does not
exist, the hub **MUST** return a `404` HTTP status code.

If the requested subscription is no longer active, the hub **MAY** either return the JSON-LD
document with the `active` property set to `false` or return a `404` status code. Likewise,
collection endpoints **MAY** include terminated subscriptions with `active` set to `false` or
omit them.

Collection endpoints **MUST** return JSON-LD documents containing at least the following
properties:

*   `@context`: the fixed value `https://mercure.rocks/`. `@context` **MAY** be omitted if
    already defined in a parent node. See (#json-ld-context).
*   `id`: the URL used to retrieve the document.
*   `type`: the fixed value `Subscriptions`.
*   `subscriptions`: an array of subscription documents as described in (#subscription-events).

In addition, all endpoints **MUST** set the `lastEventID` property at the root of the returned
JSON-LD document:

*   `lastEventID`: the identifier of the last event dispatched by the hub at the time of this
    request (see (#reconciliation)). The value **MUST** be `earliest` if no events have been
    dispatched yet. This value **SHOULD** be passed back to the hub when subscribing to
    subscription events to prevent data loss.

Because data returned by this web API is volatile, clients **SHOULD** validate that a cached
response is still fresh before using it.

Examples:

~~~ http
GET /.well-known/mercure/subscriptions
Host: example.com

200 OK
Content-type: application/ld+json
Link: <https://example.com/.well-known/mercure>; rel="mercure"
ETag: urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb
Cache-control: must-revalidate

{
   "@context": "https://mercure.rocks/",
   "id": "/.well-known/mercure/subscriptions",
   "type": "Subscriptions",
   "lastEventID": "urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb",
   "subscriptions": [
      {
         "id": "/.well-known/mercure/subscriptions/URLPattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6",
         "type": "Subscription",
         "matchType": "URLPattern",
         "match": "https://example.com/:selector",
         "subscriber": "urn:uuid:bb3de268-05b0-4c65-b44e-8f9acefc29d6",
         "active": true,
         "payload": {"foo": "bar"}
      },
      {
         "id": "/.well-known/mercure/subscriptions/Exact/https%3A%2F%2Fexample.com%2Fa-topic/urn%3Auuid%3A1e0cba4c-4bcd-44f0-ae8a-7b76f7ef1280",
         "type": "Subscription",
         "match": "https://example.com/a-topic",
         "matchType": "Exact",
         "subscriber": "urn:uuid:1e0cba4c-4bcd-44f0-ae8a-7b76f7ef1280",
         "active": true,
         "payload": {"baz": "bat"}
      },
      {
         "id": "/.well-known/mercure/subscriptions/URLPattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Aa6c49794-5f74-4723-999c-3a7e33e51d49",
         "type": "Subscription",
         "matchType": "URLPattern",
         "match": "https://example.com/:selector",
         "subscriber": "urn:uuid:a6c49794-5f74-4723-999c-3a7e33e51d49",
         "active": true,
         "payload": {"foo": "bap"}
      }
   ]
}
~~~

~~~ http
GET /.well-known/mercure/subscriptions/URLPattern/https%3A%2F%2Fexample.com%2F%3Aselector
Host: example.com

200 OK
Content-type: application/ld+json
Link: <https://example.com/.well-known/mercure>; rel="mercure"
ETag: urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb
Cache-control: must-revalidate

{
   "@context": "https://mercure.rocks/",
   "id": "/.well-known/mercure/subscriptions/URLPattern/https%3A%2F%2Fexample.com%2F%3Aselector",
   "type": "Subscriptions",
   "lastEventID": "urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb",
   "subscriptions": [
      {
         "id": "/.well-known/mercure/subscriptions/URLPattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6",
         "type": "Subscription",
         "match": "https://example.com/:selector",
         "matchType": "URLPattern",
         "subscriber": "urn:uuid:bb3de268-05b0-4c65-b44e-8f9acefc29d6",
         "active": true,
         "payload": {"foo": "bar"}
      },
      {
         "id": "/.well-known/mercure/subscriptions/URLPattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Aa6c49794-5f74-4723-999c-3a7e33e51d49",
         "type": "Subscription",
         "match": "https://example.com/:selector",
         "matchType": "URLPattern",
         "subscriber": "urn:uuid:a6c49794-5f74-4723-999c-3a7e33e51d49",
         "active": true,
         "payload": {"foo": "bap"}
      }
   ]
}
~~~

~~~ http
GET /.well-known/mercure/subscriptions/URLPattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6
Host: example.com

200 OK
Content-type: application/ld+json
Link: <https://example.com/.well-known/mercure>; rel="mercure"
ETag: urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb
Cache-control: must-revalidate

{
   "@context": "https://mercure.rocks/",
   "id": "/.well-known/mercure/subscriptions/URLPattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6",
   "type": "Subscription",
   "match": "https://example.com/:selector",
   "matchType": "URLPattern",
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
   "matchType": "mercure:matchType",
   "match": "mercure:match",
   "subscriber": "mercure:subscriber",
   "active": "mercure:active",
   "payload": "mercure:payload",
   "lastEventID": "mercure:lastEventID"
}
}
~~~

# Discovery

## Hub Discovery

The discovery mechanism aims at identifying the URL of one or more hubs designated by the publisher.

The URL of the hub **MUST** be the "well-known" [@!RFC5785] fixed path `/.well-known/mercure`.

If the publisher is a server, it **SHOULD** advertise the URL of one or more hubs to the
subscriber so that the subscriber can receive live updates. If more than one hub URL is
specified, the publisher **MUST** notify each hub, and the subscriber **MAY** subscribe to one
or more of them.

Note: Publishers may wish to advertise and publish to more than one hub for fault tolerance and
redundancy. If one hub fails to propagate an update, the others increase the likelihood of
delivery to subscribers.

The publisher **SHOULD** include at least one Link Header [@!RFC5988] with `rel=mercure` (a hub
link header). The target URL of such links **MUST** be a hub implementing the Mercure protocol.

The publisher **MAY** provide the following target attributes in the Link Headers:

*   `last-event-id`: the identifier of the last event dispatched by the publisher at the time
    the resource was generated. If provided, it **MUST** be passed to the hub through a query
    parameter named `lastEventID`; this ensures that updates dispatched between the resource
    generation and the connection to the hub are not lost. See (#reconciliation).
*   `content-type`: the content type of the updates that will be pushed by the hub. If omitted,
    the subscriber **MUST** assume that the content type matches that of the original resource.
    The `content-type` attribute is especially useful to indicate that partial updates will be
    pushed, in formats such as JSON Patch [@RFC6902] or JSON Merge Patch [@RFC7386].
*   `key-set`: the URL of the key set used to decrypt updates, encoded in the JWK Set (JSON Web
    Key Set) format [@!RFC7517]. See (#encryption). Because this key set contains a secret key,
    the publisher **MUST** ensure that only the subscriber can access this URL. The authorization
    mechanism (see (#authorization)) can be reused for that purpose.

All these attributes are optional.

Minimal example:

~~~ http
GET /books/foo
Host: example.com

200 OK
Content-type: application/ld+json
Link: <https://example.com/.well-known/mercure>; rel="mercure"

{"@id": "/books/foo", "foo": "bar"}
~~~

## Topic Discovery

The discovery mechanism **MAY** also be used to identify the canonical URL for the topic that
subscribers are expected to use for subscriptions.

The publisher **MAY** include one Link Header [@!RFC5988] with `rel=self` (the self link
header). It **SHOULD** contain the canonical URL for the topic. If the link with `rel=self` is
omitted, the current URL of the resource **MAY** be used as a fallback.

Links embedded in HTML or XML documents as defined in the WebSub recommendation
[@W3C.REC-websub-20180123] **MAY** also be supported by subscribers. If both a header and an
embedded link are provided, the header **MUST** be preferred.

### Content Negotiation

For practical purposes, the `rel=self` URL **SHOULD** offer a single representation. The hub has
no way to know which Media Type ([@RFC6838]) or language was requested by the subscriber upon
discovery, and therefore cannot select a representation on its behalf.

Content negotiation can, however, be performed by returning a different `rel=self` URL based on
the HTTP headers of the discovery request. For example, a request to `/books/foo` with an
`Accept` header containing `application/ld+json` could return a `rel=self` value of
`/books/foo.jsonld`.

The example below illustrates how a topic URL can return different `Link` headers depending on
the `Accept` header.

~~~ http
GET /books/foo
Host: example.com
Accept: application/ld+json

200 OK
Content-type: application/ld+json
Link: </books/foo.jsonld>; rel="self"
Link: <https://example.com/.well-known/mercure>; rel="mercure"

{"@id": "/books/foo", "foo": "bar"}
~~~

~~~ http
GET /books/foo
Host: example.com
Accept: text/html

200 OK
Content-type: text/html
Link: </books/foo.html>; rel="self"
Link: <https://example.com/.well-known/mercure>; rel="mercure"

<!doctype html>
<link rel="self" href="/books/foo.html">
<link rel="mercure" href="https://example.com/.well-known/mercure">
<title>foo: bar</title>
~~~

The same technique can be used to return a different `rel=self` URL depending on the language
requested by the `Accept-Language` header.

~~~ http
GET /books/foo
Host: example.com
Accept: application/ld+json
Accept-Language: fr-FR

200 OK
Content-type: application/ld+json
Content-Language: fr-FR
Link: </books/foo-fr-FR.jsonld>; rel="self"
Link: <https://example.com/.well-known/mercure>; rel="mercure"

{"@id": "/books/foo", "foo": "bar", "@context": {"@language": "fr-FR"}}
~~~

# Encryption

Using HTTPS does not prevent the hub from accessing the content of an update. Depending on the
intended privacy of the information contained in the update, it **MAY** be necessary to prevent
eavesdropping by the hub.

To prevent the hub from reading the message content, the publisher **MAY** encrypt the message
before sending it. The publisher **SHOULD** use JSON Web Encryption [@!RFC7516] to encrypt the
content of the update. The publisher **MAY** provide the URL of the relevant encryption key(s)
in the `key-set` attribute of the `Link` HTTP header during discovery; see (#discovery). The
`key-set` attribute **MUST** link to a key encoded using the JSON Web Key Set [@!RFC7517]
format. Any other out-of-band mechanism **MAY** be used instead to share the key between the
publisher and the subscriber.

Update encryption is considered a best practice to prevent mass surveillance, especially when
the hub is managed by an external provider.

# IANA Considerations

## Well-Known URIs Registry

A new "well-known" URI as described in (#discovery) has been registered in the "Well-Known URIs"
registry as described below:

*   URI Suffix: mercure
*   Change Controller: IETF
*   Specification document(s): This specification, (#discovery)
*   Related information: N/A

## Link Relation Types Registry

A new "Link Relation Type" as described in (#discovery) has been registered in the "Link Relation
Type" registry with the following entry:

*   Relation Name: mercure
*   Description: The Mercure Hub to use to subscribe to updates of this resource.
*   Reference: This specification, (#discovery)

## JSON Web Token (JWT) Registry

A new "JSON Web Token Claim" as described in (#authorization) is to be registered in the
"JSON Web Token Claims" registry with the following entry:

*   Claim Name: mercure
*   Description: Mercure data.
*   Reference: This specification, (#authorization)

# Security Considerations

The confidentiality of the secret key(s) used to generate JWSs is a primary concern. Such keys
**MUST** be stored securely and **MUST** be revoked immediately in the event of a breach.

A valid JWS allows any client that holds it to subscribe to or publish on the hub. Their
confidentiality **MUST** therefore be ensured: JWSs **MUST** only be transmitted over secure
connections.

When the client is a web browser, the JWS **SHOULD NOT** be exposed to JavaScript, to provide
resilience against [Cross-site Scripting (XSS) attacks](https://owasp.org/www-community/attacks/xss/).
For this reason, `HttpOnly` cookies **SHOULD** be preferred as the authorization mechanism in
that case.

In the event of a breach, revoking JWSs before their expiration is often difficult. Short-lived
tokens are therefore strongly **RECOMMENDED**.

The hub's publishing endpoint can be targeted by [Cross-Site Request Forgery (CSRF) attacks](https://owasp.org/www-community/attacks/csrf)
when the cookie-based authorization mechanism is used. Implementations supporting that
mechanism **MUST** mitigate such attacks.

The first preventive measure is to set the `SameSite` attribute on the `mercureAuthorization`
cookie. Because [some web browsers still do not support this
attribute](https://caniuse.com/#feat=same-site-cookie-attribute), hub implementations **SHOULD**
also use the `Origin` and `Referer` HTTP headers to verify that the source origin matches the
target origin. If neither header is available, the hub **SHOULD** reject the request.

CSRF prevention techniques are described in depth in [OWASP's Cross-Site Request Forgery (CSRF)
Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html).

JWSs **SHOULD NOT** be passed in page URLs (for example, via the `authorization` query string
parameter). Browsers, web servers, and other software may not adequately secure URLs stored in
browser history, server logs, and other data structures, and an attacker able to read those
locations could steal the token.

# Implementation Status

[RFC Editor Note: Please remove this entire section prior to publication as an RFC.]

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

Dunglas Services SAS

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

Kévin Dunglas, [contact@mercure.rocks](mailto:contact@mercure.rocks) <https://mercure.rocks>

Interoperability:

Reported compatible with all major browsers and server-side tools.

## Freddie

Implementation Name and Details:

Freddie, <https://github.com/bpolaszek/freddie>

Brief Description:

Freddie is a PHP implementation of the Mercure Hub Specification.

Level of Maturity:

Stable.

Coverage:

All the features of the protocol except the subscription events.

Version compatibility:

The implementation follows the latest draft.

Licensing:

All code is covered under the GNU General Public License v3.0.

Contact Information:

<https://github.com/bpolaszek/freddie>

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

dart*mercure, available at <<https://github.com/wallforfry/dart>*mercure>

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
recommendation [@W3C.REC-websub-20180123]. The editor wishes to thank all the authors of this
specification.

<reference anchor="urlpattern" target="https://urlpattern.spec.whatwg.org">
    <front>
        <title>URL Pattern Living Standard</title>
        <author>
            <organization>The Web Hypertext Application Technology Working Group (WHATWG)</organization>
        </author>
        <date year="2024"/>
    </front>
</reference>

<reference anchor="eventsource-interface" target="https://html.spec.whatwg.org/#the-eventsource-interface">
    <front>
        <title>HTML Living Standard</title>
        <author>
            <organization>The Web Hypertext Application Technology Working Group (WHATWG)</organization>
        </author>
        <date year="2024"/>
    </front>
</reference>

<reference anchor="xhr" target="https://xhr.spec.whatwg.org/">
    <front>
        <title>XMLHttpRequest Living Standard</title>
        <author>
            <organization>The Web Hypertext Application Technology Working Group (WHATWG)</organization>
        </author>
        <date year="2024"/>
    </front>
</reference>

<reference anchor="cel" target="https://cel.dev/">
    <front>
        <title>Common Expression Language</title>
        <author>
            <organization>Google</organization>
        </author>
        <date year="2024"/>
    </front>
</reference>

{backmatter}
