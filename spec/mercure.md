%%%
title = "The Mercure Protocol"
abbrev = "Mercure"
ipr = "trust200902"
area = "Web and Internet Transport"
workgroup = "HTTP"
submissiontype = "IETF"

[seriesInfo]
name = "Internet-Draft"
value = "draft-dunglas-mercure-08"
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

# Introduction

Mercure is a protocol for pushing updates of web resources to clients over HTTP. It builds on
Server-Sent Events [@!W3C.REC-eventsource-20150203] for delivery and on JSON Web Signatures
[@!RFC7515] for authorization, so that it can be implemented on top of existing HTTP
infrastructure and consumed natively by web browsers.

Publishers send updates to a hub. Subscribers open a long-lived HTTP connection to the hub and
declare, using topic matchers, which topics they want to receive. The hub checks authorization
and dispatches matching updates, including updates marked as private that only authorized
subscribers may receive.

This document specifies the subscription and publication interfaces, the topic matcher types,
the OAuth 2.0-based authorization model, reconnection and state reconciliation,
active-subscription tracking, discovery, and update encryption.

# Terminology

The key words **MUST**, **MUST NOT**, **REQUIRED**, **SHALL**, **SHALL NOT**, **SHOULD**,
**SHOULD NOT**, **RECOMMENDED**, **NOT RECOMMENDED**, **MAY**, and **OPTIONAL** in this document
are to be interpreted as described in BCP 14 [@!RFC2119] [@!RFC8174] when, and only when, they
appear in all capitals, as shown here.

*   Topic: The unit to which one can subscribe for changes. The topic is identified by a string
    that **MAY** be an IRI [@!RFC3987]. Topic strings **MUST** be valid UTF-8 [@!RFC3629] and
    **MUST NOT** contain C0 control characters (U+0000–U+001F) or U+007F (DEL). An update is
    about exactly one topic.
*   Update: The message containing the updated version of the topic. An update can be marked as
    private; in that case, it **MUST** be dispatched only to subscribers allowed to receive it.
*   Topic matcher: An expression matched against one or more topics,
    depending on the matcher type.
*   Topic matcher type: The type of a matching expression, either exact match or URL pattern.
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
    can use it. The hub is an OAuth 2.0 protected resource [@!RFC6749].
*   Access token: The credential a client presents to the hub to prove authorization, carried as
    a JWT [@!RFC7519] following the JWT access token profile [@!RFC9068].
*   Resource identifier: The OAuth 2.0 resource identifier of the hub, used as the audience
    (`aud`) of access tokens and advertised through protected resource metadata [@!RFC9728].
*   Authorization server: An OAuth 2.0 authorization server [@!RFC6749] that issues access tokens
    for the hub. Its use is **OPTIONAL**; access tokens **MAY** be self-issued.
*   Authorization details: The `authorization_details` claim [@!RFC9396] carried in an access
    token, expressing which actions a client may perform on which topics.

# Subscription

The subscriber subscribes to a URL exposed by a hub to receive updates from one or more topics.
To subscribe, the client opens an HTTPS connection to the hub's subscription URL (advertised by
the publisher; see (#discovery)) following the Server-Sent Events specification
[@!W3C.REC-eventsource-20150203]. The `GET` HTTP method **MUST** be used. The connection
**SHOULD** use HTTP version 2 or higher to leverage multiplexing and other performance-related
features.

The subscriber specifies the topics to receive updates from using topic matcher query
parameters. The `topic` parameter selects the `Exact` matcher type; the `topicURLPattern`
parameter selects the `URLPattern` matcher type. A request **MAY** contain several such
parameters, in any combination. See (#matcher-types). These parameters select which topics the
subscriber receives; they do not by themselves grant access to private updates, which is
governed by the access token (see (#authorization)).

The names of topic matcher query parameters are case-sensitive. A request using a topic matcher
query parameter name other than `topic` or `topicURLPattern` **MUST** be rejected with a 400
"Bad Request" HTTP status code.

The value of each topic matcher query parameter **MUST** be valid UTF-8 [@!RFC3629] and
**MUST NOT** contain C0 control characters or U+007F. Requests violating this constraint
**MUST** be rejected with a 400 "Bad Request" HTTP status code.

The subscriber receives updates for all topics matching at least one topic matcher according to
the matcher type rules.

To mitigate resource exhaustion, hubs **SHOULD** apply implementation-defined maximums to the
number of topic matcher query parameters in a single request and to the length of each
matcher's pattern. Requests exceeding any such limit **MUST** be rejected with a 400 "Bad
Request" HTTP status code. A subscription is created for every topic matcher query parameter
present in the request. Hubs **MAY** deduplicate subscriptions that have identical matcher type
and pattern. See (#subscription-events).

The `EventSource` JavaScript interface [@eventsource-interface] **MAY** be used to establish
the connection. Any other appropriate mechanism, including but not limited to readable streams
[@W3C.NOTE-streams-api-20161129] and XMLHttpRequest [@xhr] (used by popular polyfills),
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

Web Linking [@!RFC8288] **MAY** be used to indicate the IRI of the resource sent in the event.
When using Atom, XML, or HTML as the serialization format, the document **SHOULD** contain a
`link` element with a `self` relation that holds the IRI of the resource. When using JSON-LD,
the document **SHOULD** contain an `@id` property holding the IRI of the resource.

Example:

~~~ javascript
// The subscriber subscribes to updates
// for the https://example.com/foo topic, the bar topic,
// and to any topic matching the https://example.com/bar/:id URL Pattern
const url = new URL('https://example.com/.well-known/mercure');
url.searchParams.append('topic', 'https://example.com/foo');
url.searchParams.append('topic', 'bar');
url.searchParams.append('topicURLPattern', 'https://example.com/bar/:id');

const eventSource = new EventSource(url);

// The callback will be called every time an update is published
eventSource.onmessage = function ({data}) {
    console.log(data);
};
~~~

The hub **MAY** require subscribers and publishers to be authenticated, and **MAY** apply extra
authorization rules not defined in this specification.

# Matcher Types

A topic matcher is an expression matched against topics; its matcher type determines how the
expression is interpreted. This document defines two matcher types, `Exact` and `URLPattern`.
Hubs **MUST** support both.

The matcher value `*` is reserved as a wildcard that matches every topic. It is recognized before
the matcher type is resolved, so it has this meaning regardless of matcher type and regardless of
whether `matchType` is supplied or defaulted (see (#topic-matcher-list)). As a consequence, a topic
whose value is exactly `*` is not addressable: no matcher can select that single topic without also
selecting every other. This mirrors the reserved wildcard characters of other publish-subscribe
systems.

## Exact Matching

The hub **MUST** support exact matching. With this matcher type, the hub **MUST** perform an
exact, case-sensitive, byte-for-byte comparison between the topic and the matcher. The hub
**MUST NOT** resolve relative values against the hub's URL or any other base, and **MUST NOT**
perform Unicode or IRI normalization.

Note: Because comparison is performed on raw bytes, publishers and subscribers **SHOULD**
normalize topic strings to a canonical form before publication or subscription. Recommended
canonicalizations are Unicode NFC [@!UNICODE] and, for IRIs, IDNA-canonical hosts [@!RFC5891]
and percent-encoding normalization [@!RFC3986]. Otherwise, visually identical topics will be
treated as distinct, and homograph attacks (see (#security-considerations)) become possible.

The matcher type name is `Exact`. The corresponding subscribe query parameter is `topic`, and
the corresponding `matchType` value in authorization details (see (#topic-matcher-list)) is
`Exact`.

## URL Pattern

The hub **MUST** support using URL patterns [@!urlpattern] as matchers.

URL patterns **MAY** be absolute (e.g., `https://example.com/books/:id`) or relative
(e.g., `/.well-known/mercure/subscriptions/Exact/:topic/:subscriber`). When evaluating
a relative pattern or a relative topic, the hub **MUST** use the hub's URL as the
base URL. This allows subscribers to match relative topics published by the hub
itself, such as subscription events (see (#subscription-events)).

URL patterns are evaluated per the URL Pattern Living Standard [@!urlpattern]; hubs **MUST NOT**
enable the `ignoreCase` option. Host components remain case-insensitive as defined by URL
canonicalization [@!RFC3986]; all other components are case-sensitive.

The URL Pattern Living Standard compiles patterns to regular expressions internally; crafted
patterns can therefore trigger catastrophic backtracking. To mitigate denial-of-service attacks
by clients submitting pathological patterns, hubs **MUST** either use a regular expression
engine that guarantees linear-time matching (such as RE2 [@re2]) or enforce an
implementation-defined evaluation cost or time limit. When such a limit is reached, the pattern
**MUST** be treated as not matching and the evaluation **MUST** be aborted.

URL patterns whose `protocol` component is a wildcard or capture group can match `data:`,
`javascript:`, `file:`, and other potentially dangerous URI schemes. Topic strings are opaque
identifiers within this protocol; subscribers **MUST NOT** dereference them as URLs without
validating the scheme against an allowlist appropriate for the subscriber's environment.

The matcher type name is `URLPattern`. The corresponding subscribe query parameter is
`topicURLPattern`, and the corresponding `matchType` value in authorization details (see
(#topic-matcher-list)) is `URLPattern`.

## Summary of Matcher Types

| Matcher Type   | Subscribe Query Parameter | `matchType` | Requirement  |
|----------------|---------------------------|-------------|--------------|
| `Exact`        | `topic`                   | `Exact`     | **MUST**     |
| `URLPattern`   | `topicURLPattern`         | `URLPattern`| **MUST**     |

# Publication

The publisher sends updates by issuing `POST` HTTPS requests to the hub URL. When it receives an
update, the hub dispatches it to subscribers using the established server-sent events connections.

The hub **MAY** also dispatch the update using other protocols such as WebSub
[@W3C.REC-websub-20180123] or ActivityPub [@W3C.REC-activitypub-20180123].

An application **MAY** deliver events directly to subscribers without an external hub. In that
case, the publish endpoint described in this section is not required.

The request **MUST** be encoded using the `application/x-www-form-urlencoded` format
[@W3C.REC-html52-20171214] and **MUST** contain exactly one `topic` field. Field names and
values **MUST** be UTF-8 [@!RFC3629]. It **MAY** also contain the following name-value tuples:

*   `topic`: The identifier of the updated topic, and the resource against which private-read
    authorization is evaluated (see (#subscribers)). It is **RECOMMENDED** to use an IRI as
    identifier. This field **MUST** appear exactly once; a request carrying more than one `topic`
    field **MUST** be rejected with a 400 "Bad Request" HTTP status code. The topic value
    **MUST** conform to the constraints defined in (#terminology). The topic **MUST NOT** address
    the reserved hub namespace. A topic addresses the reserved namespace when, after being
    resolved as a URI reference against the hub's URL, its path component is `/.well-known/mercure`
    or begins with `/.well-known/mercure/`, regardless of scheme or authority. This namespace is
    reserved for resources generated by the hub itself, including subscription events (see
    (#subscription-events)). Hubs **MUST** reject publish requests violating this rule with a 403
    HTTP status code. Checking the resolved path component (rather than a leading-substring match
    on the raw value) prevents a publisher from forging subscription events with an absolute topic
    such as `https://hub.example.com/.well-known/mercure/subscriptions/...`.
*   `data` (optional): the content of the new version of this topic. The value **MUST** be
    valid UTF-8 [@!RFC3629].
*   `private` (optional): if this field is present, the update **MUST NOT** be dispatched to
    subscribers not authorized to receive it. See (#authorization). The presence of the field
    name marks the update as private regardless of its value, whether or not a value is
    supplied; hubs **MUST NOT** interpret the field's value to determine privacy. It is
    **RECOMMENDED** to set the value to `on` for interoperability, but it **MAY** contain any
    value, including an empty string.
*   `id` (optional): the topic's revision identifier; used as the SSE `id` property.
    The provided ID **MUST NOT** start with the `#` character and **MUST NOT** contain U+000A
    (LF), U+000D (CR), or U+0000 (NUL). The provided ID **MAY** be a valid IRI. If omitted, the
    hub **MUST** generate a valid IRI [@!RFC3987]. A UUID [@RFC9562] or a
    [@DID] **MAY** be used. Alternatively, the hub **MAY**
    generate a relative URI composed of a fragment (starting with `#`). This is convenient to
    return an offset or a sequence that is unique for this hub. The hub **MAY** ignore the
    client-supplied ID and generate its own. The hub **MUST** reject client-supplied IDs
    violating the character constraints above with a 400 HTTP status code.
*   `type` (optional): the SSE `event` property (a specific event type). The value **MUST NOT**
    contain U+000A or U+000D; hubs **MUST** reject violating values with a 400 HTTP status
    code.
*   `retry` (optional): the SSE `retry` property (the reconnection time). The value **MUST**
    consist solely of ASCII digits (U+0030–U+0039); hubs **MUST** reject violating values with
    a 400 HTTP status code.

On success, the hub **MUST** return a 2xx HTTP status code, and the response body **MUST** be
the `id` generated by the hub for the update. Hubs **SHOULD** use 200 (OK). The status code 201
(Created) is **NOT RECOMMENDED**: an update is an ephemeral message rather than a resource
retrievable at a dereferenceable URL, so the `id` is an event cursor (see (#reconciliation)),
not a `Location`. The publisher **MUST** be authorized to publish updates; see (#authorization).

Hubs **SHOULD** apply implementation-defined maximums to the size of the request body and to
the length of individual fields. Requests exceeding any such limit **MUST** be rejected with a
413 "Content Too Large" HTTP status code.

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

The hub is an OAuth 2.0 protected resource [@!RFC6749]. To prove that they are authorized, both
publishers and subscribers **MUST** present an access token to the hub. The access token
**MUST** be a JWT [@!RFC7519] following the JWT access token profile [@!RFC9068], carried as a
JWS [@!RFC7515] in compact serialization. The token **SHOULD** be short-lived, especially when
the subscriber is a web browser.

Access tokens **MAY** be issued by an OAuth 2.0 authorization server or self-issued by the
publisher (for example, signed with a key shared out of band with the hub). The hub need not
operate or trust an external authorization server. When an authorization server is used, the hub
**MAY** advertise it through protected resource metadata (see (#discovery)). Different keys
**SHOULD** be used to sign subscribers' and publishers' tokens so that compromise of one role
does not entail compromise of the other.

Authorization is expressed with the `authorization_details` claim [@!RFC9396]; see
(#authorization-details). Routing (which topics a subscriber listens to, via the query
parameters of (#subscription)) is independent of authorization (what a token permits): the query
parameters never grant access to private updates.

Note: Hubs **MAY** be deployed without requiring authorization (for example, when serving only
publicly-readable updates over a trusted network). Such deployments fall outside the scope of
the rest of this section. They **MUST NOT** be reachable from networks containing untrusted
clients, since any client able to reach the hub will be able to publish and subscribe at will.
The remainder of this section assumes token-based authorization is in use.

## Presenting the Access Token

Three mechanisms are defined to present the access token to the hub, following the OAuth 2.0
Bearer Token Usage specification [@!RFC6750] where applicable:

*   an `Authorization` HTTP header with the `Bearer` scheme [@!RFC6750],
*   a cookie (a Mercure extension for web browsers, see below),
*   an `access_token` URI query parameter [@!RFC6750].

When any of these mechanisms is used, the connection **MUST** use an encryption layer such as
HTTPS.

When more than one mechanism is present, the hub **MUST** select exactly one token using the
following precedence, from highest to lowest: the `Authorization` HTTP header, then the
`access_token` query parameter, then the cookie. The token from the selected mechanism **MUST**
be used and the others **MUST** be ignored. Concretely: if an `Authorization` HTTP header is
present, its token **MUST** be used and the `access_token` query parameter and the cookie **MUST**
be ignored; otherwise, if an `access_token` query parameter is present, it **MUST** be used and the
cookie **MUST** be ignored; otherwise, the cookie, if any, **MUST** be used.

### Authorization HTTP Header

If the publisher or the subscriber is not a web browser, it **SHOULD** use an `Authorization`
HTTP header. This header **MUST** contain the string `Bearer` followed by a space character and
by the access token, as defined in [@!RFC6750].

### Cookie

Per the `EventSource` specification [@W3C.REC-eventsource-20150203], web browsers cannot set
custom HTTP headers on such connections, and the connections can only be established using the
`GET` HTTP method. However, cookies are supported and can be included even in cross-domain
requests if [the CORS credentials are
set](https://html.spec.whatwg.org/multipage/server-sent-events.html#dom-eventsourceinit-withcredentials).
This cookie mechanism is a Mercure-specific extension to [@!RFC6750]; hubs that support it
**SHOULD** advertise it in their protected resource metadata (see (#discovery)).

If the publisher or the subscriber is a web browser, it **SHOULD**, whenever possible, send a
cookie containing the access token when connecting to the hub. It is **RECOMMENDED** to name the
cookie `mercureAccessToken`, but a different name **MAY** be used to prevent conflicts when
several hubs share the same domain.

The cookie **SHOULD** be set during discovery (see (#discovery)) to improve overall security.
Consequently, if the cookie is set during discovery, the publisher and the hub **MUST** share
the same registrable domain (eTLD+1). The `Domain` attribute **MAY** be used to allow the
publisher and the hub to use different subdomains of that registrable domain. See (#discovery).

The cookie **MUST** have the `Secure` and `HttpOnly` attributes set. The cookie **SHOULD** also
have `SameSite=Strict`; `SameSite=Lax` **MAY** be used if cross-site discovery flows require it.
The cookie's `Path` attribute **SHOULD** be set to the path of the hub's subscription URL. See
(#security-considerations).

### URI Query Parameter

If the client cannot use an `Authorization` HTTP header or a cookie, the access token **MAY** be
passed in the `access_token` URI query parameter as defined in [@!RFC6750] section 2.3.

For example, the client makes the following HTTP request using transport-layer security:

~~~ http
GET /.well-known/mercure?topic=https://example.com/books/foo&access_token=<token>
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

## Error Responses

The hub reports authorization failures using the error responses defined in [@!RFC6750]
section 3:

*   If no access token is presented, the hub **MUST** return a 401 "Unauthorized" status code
    with a `WWW-Authenticate: Bearer` challenge and **MUST NOT** include an error code. The
    challenge **SHOULD** include a `resource_metadata` parameter pointing to the hub's protected
    resource metadata (see (#discovery)) per [@!RFC9728].
*   If a token is presented but fails validation as defined in (#token-validation), the hub
    **MUST** return a 401 status code with `error="invalid_token"`.
*   If a valid token does not authorize the requested operation, the hub **MUST** return a 403
    "Forbidden" status code with `error="insufficient_scope"`.
*   If the request is malformed, the hub **MUST** return a 400 "Bad Request" status code with
    `error="invalid_request"`.

Returning `invalid_token` for every presented-token failure (rather than distinguishing a bad
signature from an expired or not-yet-valid token) avoids disclosing why validation failed.

## Token Validation {#token-validation}

Hubs **MUST** validate access tokens as JWT access tokens [@!RFC9068] and in accordance with the
JSON Web Token Best Current Practices [@!RFC8725]. In particular:

*   Hubs **MUST** verify that the token header `typ` is `at+jwt` [@!RFC9068], so that tokens
    issued for other purposes (for example, OpenID Connect ID Tokens) are not accepted.
*   Hubs **MUST** be configured with an explicit allowlist of accepted signature algorithms and
    **MUST** reject any token whose `alg` is not on that allowlist. Hubs **MUST NOT** accept
    `alg=none`, **MUST NOT** derive the set of acceptable algorithms from the token, and **MUST**
    verify that `alg` is compatible with the key used for verification (preventing
    algorithm-confusion attacks). The allowlist **SHOULD** include at minimum `EdDSA`, `ES256`,
    and `RS256`, and **MUST NOT** include any algorithm whose security has been compromised at
    the time of deployment.
*   Hubs **MUST** select the verification key independently of attacker-controlled input. When
    more than one key is in use — for example, separate publisher and subscriber keys or rotated
    keys — the hub **SHOULD** select the key using the `kid` header parameter and/or the role of
    the endpoint, and **MUST NOT** allow the token to cause an unexpected key to be selected.
    Verification keys are obtained from static configuration, from the authorization server's JWK
    Set [@!RFC7517] discovered through its metadata [@!RFC8414], or from the hub's protected
    resource metadata (see (#discovery)). This specification defines no hub-specific key
    distribution endpoint.
*   Hubs **MUST** enforce the `exp` claim [@!RFC7519], including on the first request received
    bearing a token, and **MUST** enforce the `nbf` claim if present.
*   Hubs **MUST** be configured with their resource identifier and **MUST** verify that it
    appears in the token `aud` claim [@!RFC9068]. Per [@!RFC7519], `aud` **MAY** be a single
    string or an array of strings; the resource identifier matches when it equals that string or
    is a member of that array. This bounds a token to its intended hub and mitigates replay across
    hubs that share signing keys (see (#hub-trust)). When the hub advertises one or more
    authorization servers in its protected resource metadata (see (#discovery)), it **MUST**
    verify that the `iss` claim is the issuer identifier of one of them [@!RFC9068]. A hub that
    accepts self-issued tokens and advertises no authorization server **MAY** omit the `iss`
    check.

The `sub` claim, when present, identifies the subscriber and is used to derive subscription
event identifiers (see (#subscription-events)).

Failure of any of these checks **MUST** be reported as defined in (#error-responses).

## Authorization Details {#authorization-details}

Authorization is expressed with the `authorization_details` claim [@!RFC9396], a JSON array of
authorization detail objects. This specification defines the authorization detail type `mercure`.

A `mercure` authorization detail object:

*   **MUST** have a `type` property whose value is the string `mercure`.
*   **MUST** have an `actions` property: a non-empty JSON array whose values are a subset of
    `["publish", "subscribe"]` (RFC 9396 `actions` field).
*   **MUST** have a `topics` property: a non-empty JSON array of topic matcher objects (see
    (#topic-matcher-list)) identifying the topics the actions apply to.
*   **MAY** have a `payload` property (a JSON object), meaningful only for the `subscribe` action
    (see (#payloads)).

A token grants an action on a topic when it carries a `mercure` authorization detail whose
`actions` includes that action and one of whose `topics` matches the topic. Tokens with no
`mercure` authorization detail grant no publish or subscribe rights.

Hubs **SHOULD** apply implementation-defined maximums to the number of `mercure` authorization
details, to the number of entries in each `topics` array, and to the length of individual
patterns. Tokens exceeding any such limit **MUST** be rejected with a 400 status code. If any
`mercure` authorization detail fails to parse or validate (including failures specific to a
matcher's `matchType`), the hub **MUST** reject the request with a 400 status code and **MUST
NOT** act on the basis of the remaining entries; partial acceptance is forbidden because it
would silently alter the effective authorization of the token.

Hubs **MAY** also limit the number of concurrent subscriptions established under a single token
and **MAY** reject further subscription attempts with a 429 "Too Many Requests" status code once
the limit is reached.

## Publishers

A publisher **MUST** present a token that grants the `publish` action on the update's topic: the
token **MUST** carry a `mercure` authorization detail whose `actions` includes `publish` and one
of whose `topics` matches the update's topic. Otherwise the hub **MUST NOT** dispatch the update
and **MUST** return a 403 status code with `error="insufficient_scope"` (see (#error-responses)).

## Subscribers

To receive updates marked as `private`, a subscriber **MUST** present a token that grants the
`subscribe` action on the update's topic: the token **MUST** carry a `mercure` authorization
detail whose `actions` includes `subscribe` and one of whose `topics` matches the update's topic.
If the token does not grant `subscribe` on that topic, the hub **MUST NOT** deliver the update to
the subscriber.

Because an update has exactly one topic (see (#publication)) and authorization is evaluated
against that single topic, a subscriber receives a private update only when its token explicitly
grants `subscribe` on that resource. The subscriber's routing matchers (query parameters) only
select which topics it listens to; they never widen what it may read.

Because `exp` is required (see (#token-validation)), the hub **MUST** close the connection no
later than the token's `exp` time. Since `exp` alone cannot revoke an already-established
long-lived connection, hubs **SHOULD** also impose a maximum connection lifetime independent of
`exp` and close connections that exceed it, requiring the subscriber to reconnect and
re-authenticate.

For example, a subscriber may listen to all books via the routing matcher
`topicURLPattern=https://example.com/books/:id` while its token authorizes reading only specific
books:

~~~ json
{
  "authorization_details": [
    {
      "type": "mercure",
      "actions": ["subscribe"],
      "topics": [
        {"match": "https://example.com/books/1", "matchType": "Exact"},
        {"match": "https://example.com/books/7", "matchType": "Exact"}
      ]
    }
  ]
}
~~~

A private update for `https://example.com/books/1` is delivered; a private update for
`https://example.com/books/2` is not, even though the routing matcher would select it.

## Topic Matcher List

A topic matcher object appears in the `topics` array of a `mercure` authorization detail (see
(#authorization-details)). It **MUST** be a JSON object with a `match` property containing the
topic matcher itself, and **MAY** have an OPTIONAL `matchType` property containing the matcher
type. The value of `matchType` is case-sensitive and **MUST** be either `Exact` or `URLPattern`
(see (#matcher-types)). If no `matchType` key is present, the hub **MUST** assume the `Exact`
matcher type. A `match` value of `*` is the reserved wildcard and matches every topic regardless
of `matchType`, including when `matchType` is absent (see (#matcher-types)).

Any entry that is not a JSON object, or that fails to parse or validate as a topic matcher,
**MUST** cause the request to be rejected with a 400 status code as defined in
(#authorization-details).

## Payloads

User-defined data can be attached to a subscription and made available through the subscription
API and in subscription events. See (#subscription-events).

A `mercure` authorization detail with the `subscribe` action **MAY** carry a `payload` JSON
object. The `payload` of the first `subscribe` authorization detail whose `topics` matches the
subscription's own matcher (the `topic` or `topicURLPattern` query parameter value) **MUST** be
included under the `payload` key of the JSON object describing the subscription, both in the
subscription API and in subscription events. A `subscribe` detail whose `topics` contains the `*`
wildcard matches every subscription and can serve as a default.

Note: Payload selection is order-dependent; the first matching authorization detail wins. Issuers
placing broad matchers before more specific entries will mask the payloads of the specific
entries. Specific matchers **SHOULD** appear before broader ones.

Privacy: Payloads are forwarded to other authorized subscribers via subscription events (see
(#subscription-events)). Issuers **MUST NOT** place data in payloads that should not be visible
to other subscribers authorized for the corresponding subscription events. In particular,
storing data identifying the subscriber (such as a user identifier or IP address) effectively
broadcasts that data to all other subscribers within the same subscription-events scope.

Example access token claims carrying payloads:

~~~ json
{
    "authorization_details": [
        {
            "type": "mercure",
            "actions": ["subscribe"],
            "topics": [{"match": "https://example.com/foo"}],
            "payload": {"custom1": "data only available for this topic"}
        },
        {
            "type": "mercure",
            "actions": ["subscribe"],
            "topics": [{"match": "https://example.com/bar/:id", "matchType": "URLPattern"}],
            "payload": {"custom2": "data available for matching subscriptions"}
        },
        {
            "type": "mercure",
            "actions": ["subscribe"],
            "topics": [{"match": "*"}],
            "payload": {"custom3": "default data for all other subscriptions"}
        }
    ]
}
~~~

For instance, a payload can carry coarse-grained metadata such as a tenant identifier or a
display label for the subscription. Issuers **MUST** consider the privacy note above before
including any identifier of the subscriber, since payloads are visible to other authorized
subscribers via subscription events.

# Reconnection, State Reconciliation, and Event Sourcing {#reconciliation}

The protocol allows reconciliation of state after a reconnection. It can also be used to
implement an [Event store](https://en.wikipedia.org/wiki/Event_store).

To allow re-establishment in case of connection loss, events dispatched by the hub **MUST**
include an `id` property. The value of this property **SHOULD** be an IRI [@!RFC3987]. A UUID
[@RFC9562] or a [@DID] **MAY** be used.

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
**SHOULD** send all events published after the one bearing this identifier to the subscriber,
subject to authorization.

The authorization rules defined in (#subscribers) apply to replayed events identically to live
events: the hub **MUST** re-evaluate each candidate replayed event against the current access
token before dispatching it. Events whose `private` flag is set and that the token does not
authorize the subscriber to read (see (#authorization-details)) **MUST NOT** be dispatched,
regardless of any authorization that may have applied at publication time.

The reserved value `earliest` requests that the hub send all updates it has for the subscribed
topics. The hub **MAY** ignore this request according to its own policy. Because event
identifiers are IRIs (see above), `earliest` cannot collide with a hub-generated identifier.

The hub **MAY** discard some events for operational reasons. When the request contains a
`Last-Event-ID` HTTP header or a `lastEventID` query parameter, the hub **MUST** set a
`Last-Event-ID` header on the HTTP response.

The value of this response header **MUST** be the identifier of the event preceding the first
event sent to the subscriber, or the reserved value `earliest` if there is no preceding event
(for example, when the hub history is empty, when the subscriber requests the earliest event,
or when the requested event does not exist or has been discarded).

Subscribers using the hub as an event store can use the returned identifier as a recovery
anchor; subscribers that only need to detect data loss can compare it against the requested
value (a different value indicates that loss may have occurred).

Note: Event identifiers are cursors, and the hub exposes them to subscribers, both through the
`lastEventID` value provided during discovery and through the `Last-Event-ID` response header.
The header value can be the identifier of an event immediately preceding one the subscriber is
not authorized to receive. A subscriber can therefore infer the existence of such an event and,
when the hub uses time-ordered identifiers (e.g., UUIDv7 [@!RFC9562]), its approximate timing
and ordering. Operators handling sensitive private updates **SHOULD** generate opaque, random
event identifiers (e.g., UUIDv4 [@!RFC9562]), or expose an encrypted form of an internal ordered
identifier, so that the cursor discloses nothing beyond what the subscriber already knows.

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

*   `{matchType}`: the topic matcher type used for this subscription. The value **MUST** be the
    matcher type name in its canonical case as defined in (#matcher-types) (e.g., `URLPattern`,
    `Exact`). URL path components are case-sensitive.
*   `{match}`: the topic matcher used for this subscription
*   `{subscriber}`: a unique identifier for the subscriber. Subscribers and publishers
    **MUST NOT** forge, supply, or override this value through query parameters, headers,
    request bodies, or any other client-controlled channel. The hub **MUST** derive the
    identifier exclusively from information it has cryptographically validated — typically
    the `sub` claim of the subscriber's access token (after validation per (#token-validation)), but
    other authenticated values **MAY** be used. The hub **MUST** ensure that the identifier
    is unique among active subscriptions.

Note: Because strings containing reserved characters (e.g., URIs, URL Patterns, and URI
Templates) can be used for the `{match}` and `{subscriber}` variables, per [@!RFC6570] the
value of each variable **MUST** be percent-encoded exactly once during expansion, encoding the
raw matcher or subscriber string as a whole (any `%` characters it already contains are
themselves encoded). Hubs **MUST NOT** double-encode an already-encoded value.

If a subscriber has several subscriptions, the hub **SHOULD** assign the same `{subscriber}`
value to all of them when it can correlate them (for example, when the same `sub` claim is
presented across requests).

`{subscriber}` **SHOULD** be an IRI [@!RFC3987]. A UUID [@RFC9562] or a
[@DID] **MAY** also be used.

The content of the update **MUST** be a JSON-LD [@!W3C.REC-json-ld-20140116] document containing
at least the following properties:

*   `@context`: the fixed value `https://mercure.rocks/`. `@context` **MAY** be omitted if
    already defined in a parent node. See (#json-ld-context).
*   `id`: the identifier of this update; **MUST** be the same value as the subscription update's
    topic.
*   `type`: the fixed value `Subscription`.
*   `matchType`: the topic matcher type used for this subscription. The value is case-sensitive
    and **MUST** be the matcher type name in its canonical case as defined in (#matcher-types).
*   `match`: the topic matcher used for this subscription.
*   `subscriber`: the identifier of the subscriber. It **SHOULD** be an IRI.
*   `active`: `true` when the subscription is active, `false` when it is terminated.
*   `payload` (optional): the content of the `payload` field associated with this subscription
    in the subscriber's access token (see (#payloads)).

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
(#authorization). The hub **MUST** treat the requested URL (the request target, resolved against
the hub's URL per (#url-pattern)) as the topic to authorize, and **MUST** verify that a `mercure`
authorization detail in the access token grants the `subscribe` action on it, evaluated with the
matcher rules of (#matcher-types). The same matching applies to every endpoint shape above: the
collection URL `/.well-known/mercure/subscriptions`, the per-matcher collection URL, and the
single-subscription URL are each matched as a topic, so a token whose `subscribe` matcher selects
a broader set (for example a `URLPattern` covering the subscriptions namespace, or the reserved
`*`) grants access to the corresponding endpoints. If no detail grants `subscribe` on the
requested URL, the hub **MUST** answer `403` as defined in (#authorization).

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

Subscribers parsing JSON-LD documents produced by the hub **SHOULD NOT** automatically
dereference the `@context` URL. The context below is fixed and **MAY** be embedded in client
implementations or cached locally; this avoids both unnecessary network requests and the risk
that an attacker able to control responses from the context URL alters the semantics of
subsequently parsed documents.

The JSON-LD context available at `https://mercure.rocks/` is the following:

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

The publisher **SHOULD** include at least one Link Header [@!RFC8288] with `rel=mercure` (a hub
link header). The target URL of such links **MUST** be a hub implementing the Mercure protocol.

Note: A compromised publisher can advertise a malicious hub URL and capture the access tokens of
subscribers that connect to it. Subscribers **SHOULD** restrict accepted hub URLs to origins
they have a basis to trust (for example, hubs sharing the publisher's registered domain) and
**MAY** verify the hub's identity through out-of-band means before transmitting credentials.

The publisher **MAY** provide the following target attributes in the Link Headers:

*   `last-event-id`: the identifier of the last event dispatched by the publisher at the time
    the resource was generated. If provided, it **MUST** be passed to the hub through a query
    parameter named `lastEventID`; this ensures that updates dispatched between the resource
    generation and the connection to the hub are not lost. See (#reconciliation).
*   `content-type`: the content type of the updates that will be pushed by the hub. If omitted,
    the subscriber **MUST** assume that the content type matches that of the original resource.
    The `content-type` attribute is especially useful to indicate that partial updates will be
    pushed, in formats such as JSON Patch [@RFC6902] or JSON Merge Patch [@RFC7386].

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

## Protected Resource Metadata

As an OAuth 2.0 protected resource, the hub **SHOULD** publish OAuth 2.0 Protected Resource
Metadata [@!RFC9728] at the path derived from its URL as defined in that specification (for the
hub URL `/.well-known/mercure`, the metadata is served at
`/.well-known/oauth-protected-resource/.well-known/mercure`). The metadata document **SHOULD**
include:

*   `resource`: the hub's resource identifier, which is the value clients **MUST** place in the
    access token `aud` claim (see (#token-validation)).
*   `authorization_servers` (optional): issuer identifiers of the authorization servers that can
    issue tokens for the hub [@!RFC8414]. Omitted when tokens are self-issued.
*   `jwks_uri` (optional): the location of the hub's token verification keys, when not obtained
    from an authorization server.
*   `bearer_methods_supported`: the token presentation methods the hub accepts (see
    (#presenting-the-access-token)). Standard values are `header` and `query` [@!RFC9728]. The
    cookie method is advertised under the implementation-specific value `mercureCookie`; the
    namespaced name avoids colliding with any future IANA-registered method. Because it is not an
    IANA-registered value, clients that strictly follow [@!RFC9728] ignore it and so cannot
    discover the cookie method from this field alone.

When a request carries no access token, the hub's `WWW-Authenticate: Bearer` challenge
**SHOULD** include a `resource_metadata` parameter pointing to this document (see
(#error-responses)), so that clients can discover the resource identifier and authorization
server without prior configuration.

## Topic Discovery

The discovery mechanism **MAY** also be used to identify the canonical URL for the topic that
subscribers are expected to use for subscriptions.

The publisher **MAY** include one Link Header [@!RFC8288] with `rel=self` (the self link
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
content of the update. The encryption keys are shared between the publisher and the subscriber
through any out-of-band mechanism, for example a JSON Web Key Set [@!RFC7517]; the hub is not
involved in this exchange.

Update encryption is considered a best practice to prevent mass surveillance, especially when
the hub is managed by an external provider.

Implementations **MUST** restrict JWE algorithms to those whose security properties remain
acceptable at the time of deployment. Algorithms whose security has been compromised
(for example, `RSA1_5` due to padding oracle vulnerabilities [@RFC8017]) **MUST NOT** be used.
At the time of writing, key management algorithms `ECDH-ES+A256KW` and `RSA-OAEP-256`, and
content encryption algorithm `A256GCM`, are **RECOMMENDED**.

JWE provides integrity per message but does not provide replay protection: a hub or an
on-path attacker that captures a ciphertext can later replay it without modifying it.
Publishers concerned with replay **SHOULD** include a freshness indicator (such as a timestamp
or nonce) inside the encrypted payload and require subscribers to validate it.

Long-lived JWE keys do not provide forward secrecy: compromise of such keys decrypts all past
traffic encrypted under them. Publishers handling sensitive data **SHOULD** rotate JWE keys
periodically and **MAY** use ephemeral key agreement (e.g., `ECDH-ES`) to bound the impact of
a future key compromise.

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

## Authorization Details Type

A new authorization details type as described in (#authorization-details) is to be registered in
the "OAuth Authorization Server Metadata" — "Authorization Details Type" registry established by
[@!RFC9396], with the following entry:

*   Type: mercure
*   Reference: This specification, (#authorization-details)

# Security Considerations

The confidentiality of the secret key(s) used to sign access tokens is a primary concern. Such
keys **MUST** be stored securely and **MUST** be revoked immediately in the event of a breach.

A valid access token allows any client that holds it to subscribe to or publish on the hub. Its
confidentiality **MUST** therefore be ensured: access tokens **MUST** only be transmitted over
secure connections.

When the client is a web browser, the access token **SHOULD NOT** be exposed to JavaScript, to
provide resilience against [Cross-site Scripting (XSS) attacks](https://owasp.org/www-community/attacks/xss/).
For this reason, `HttpOnly` cookies **SHOULD** be preferred as the authorization mechanism in
that case.

In the event of a breach, revoking access tokens before their expiration is often difficult.
Short-lived tokens are therefore strongly **RECOMMENDED**.

The hub's publishing endpoint can be targeted by [Cross-Site Request Forgery (CSRF) attacks](https://owasp.org/www-community/attacks/csrf)
when the cookie-based authorization mechanism is used. Implementations supporting that
mechanism **MUST** mitigate such attacks.

The first preventive measure is to set the `SameSite` attribute on the `mercureAccessToken`
cookie. Because some deployed user agents may not enforce this attribute, hub implementations
**SHOULD** also use the `Origin` and `Referer` HTTP headers to verify that the source origin
matches the target origin. If neither header is available, the hub **SHOULD** reject the
request.

CSRF prevention techniques are described in depth in [OWASP's Cross-Site Request Forgery (CSRF)
Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html).

Access tokens **SHOULD NOT** be passed in page URLs (for example, via the `access_token` query
parameter). Browsers, web servers, and other software may not adequately secure URLs stored in
browser history, server logs, and other data structures, and an attacker able to read those
locations could steal the token.

## Token Validation

Access tokens are validated as JWT access tokens before authorization details are evaluated (see
(#token-validation)). Failure to verify the `typ` is `at+jwt`, to reject `alg: none`, to bind
`alg` to the key type, to enforce `exp`/`nbf`, or to verify `aud` enables token forgery, token
type confusion (for example, accepting an ID Token), replay across contexts, and
algorithm-confusion attacks.

## Server-Sent Events Field Injection

Topic strings and the `id`, `type`, and `retry` publish fields end up on the wire as part of
the Server-Sent Events framing. Values containing CR (U+000D), LF (U+000A), or NUL (U+0000)
could inject arbitrary SSE fields into the stream as seen by subscribers, including forged
event identifiers and event types. The character constraints in (#terminology) and
(#publication) prevent this injection.

## Reserved Hub Namespace

The URL path prefix `/.well-known/mercure/` is reserved for resources generated by the hub
itself (see (#publication)). A publisher with broad scope publishing under this prefix could
forge subscription events (see (#subscription-events)) and mislead other subscribers tracking
subscription lifecycle. The reserved-namespace test is applied to the topic's path component
after resolution against the hub's URL, not as a leading-substring match on the raw value; a
substring test would let an absolute topic addressing the hub's own host (for example,
`https://hub.example.com/.well-known/mercure/subscriptions/...`) bypass it.

## Authorization on Event Replay

When a subscriber reconnects with a `Last-Event-ID` header or `lastEventID` query parameter,
the same authorization rules apply to replayed events as to live events (see (#reconciliation)
and (#subscribers)). A subscriber whose authorized scope has shrunk between publication and
reconnection does not receive private events outside its scope at reconnection time. The
`Last-Event-ID` response header is a cursor, however, and **MAY** contain the identifier of an
event the subscriber is not authorized to receive. Operators handling sensitive private updates
**SHOULD** use opaque, random event identifiers so that this identifier discloses nothing
beyond the event's existence.

## Subscriber Identifier Assignment

The `{subscriber}` identifier in subscription event topic URLs is derived from information the
hub has cryptographically validated, typically the `sub` claim of the subscriber's JWS (see
(#subscription-events)). Allowing clients to supply, suggest, or override this value through any
unauthenticated channel would enable spoofing of subscription events and hijacking of
subscription state belonging to other subscribers.

## Private Update Authorization

A private update has exactly one topic, and the hub delivers it to a subscriber only when the
subscriber's access token grants the `subscribe` action on that topic (see (#subscribers) and
(#authorization-details)). Authorization is therefore a hub-enforced, per-resource check tied to
the token issued for the subscriber, not a property of the publisher's topic construction or of
the subscriber's routing matchers. Issuers scope each token's authorization details to the
resources a subscriber may read; the hub never widens that based on routing.

## URL-Pattern Denial of Service

URL Pattern compiles internally to a regular expression. Naive implementations on engines such
as PCRE are vulnerable to catastrophic backtracking. The mitigations required in (#url-pattern)
— a linear-time engine (such as RE2 [@re2]) or a per-evaluation cost or time limit — bound this
exposure.

## Payload Privacy

Payloads carried in `mercure` authorization details are included in subscription events and
forwarded to other authorized subscribers (see (#payloads)). Within the set of subscribers
authorized for the corresponding subscription events, a payload is effectively broadcast; it
cannot carry private metadata about an individual subscriber.

## Topic Normalization

Topic strings are compared as byte sequences. Without Unicode normalization (NFC) and IDNA
host canonicalization, visually identical topics may be treated as distinct, leading to
undelivered updates or to spoofable topic names through homograph attacks (e.g.,
`example.com` versus a host containing Cyrillic look-alike characters). The normalization
guidance in (#exact-matching) addresses this.

## Resource Limits

Absent limits on request and token size, malicious clients can exhaust hub resources. The
implementation-defined limits described elsewhere in this document — on publish request body
size, individual field length, the number of topic matcher query parameters per request, the
number of `mercure` authorization details and of entries in their `topics` arrays, individual
pattern length, and concurrent subscriptions per token — bound this exposure.

## Hub Trust

Subscribers obtain hub URLs from publishers via the discovery mechanism (see (#discovery)) and
transmit credentials to the hub. A compromised publisher can therefore redirect subscribers to
a hub of its choosing and capture those credentials. As described in (#discovery), subscribers
constrain the set of hub origins they connect to and can verify hub identity out of band. Scoping
each token to its intended hub with the `aud` claim (see (#token-validation)) limits the value of
a captured token: a token bound to one hub's identifier cannot be replayed against another hub,
even when the two share signing keys.

## Protected Resource Metadata and Authorization Server Selection

When the hub advertises an authorization server through protected resource metadata [@!RFC9728]
(see (#discovery)), a client that is misled into using an inappropriate authorization server may
expose itself to an adversary-in-the-middle. Clients **SHOULD** validate protected resource
metadata as described in [@!RFC9728] and obtain it only from the deterministically derived,
TLS-protected well-known location. Hubs and clients fetching metadata or key sets by URL
**SHOULD** take precautions against server-side request forgery, such as refusing requests to
internal address ranges.

## Bearer Tokens and Sender Constraint

The access token is a bearer credential: any party that obtains it can act within its scope until
it expires. Short-lived tokens (see above) limit the exposure window but do not prevent use of a
token during its lifetime. Deployments protecting high-value operations **MAY** additionally
sender-constrain tokens, for example with DPoP [@RFC9449] or mutual-TLS-bound tokens, so that a
captured token is unusable without the corresponding proof-of-possession key.

## Publish Request Replay

A captured publish request carrying a bearer access token can be replayed by an on-path attacker,
causing the same update to be dispatched again. For most deployments re-dispatching an identical update
is harmless. Deployments for which it is not **SHOULD** include a freshness indicator in the
update (for example, a unique `id` checked for replay, or a timestamp) and reject duplicates.

## JWE Algorithms and Replay

JWE-protected updates are subject to algorithm-selection pitfalls and to replay. The algorithm
restrictions and freshness guidance in (#encryption) address these.

# Implementation Status

[RFC Editor Note: Please remove this entire section prior to publication as an RFC.]

This section records the status of known implementations of the protocol defined by this
specification at the time of posting of this Internet-Draft, and is based on a proposal described
in [@RFC7942]. The description of implementations in this section is intended to assist the IETF in
its decision processes in progressing drafts to RFCs. Please note that the listing of any individual
implementation here does not imply endorsement by the IETF. Furthermore, no effort has been spent to
verify the information presented here that was supplied by IETF contributors. This is not intended
as, and must not be construed to be, a catalog of available implementations or their features.
Readers are advised to note that other implementations may exist. According to RFC 7942, "this will
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

dart_mercure, available at <https://github.com/wallforfry/dart_mercure>

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

<reference anchor="DID" target="https://www.w3.org/TR/did-core/">
    <front>
        <title>Decentralized Identifiers (DIDs) v1.0</title>
        <author>
            <organization>World Wide Web Consortium (W3C)</organization>
        </author>
        <date year="2022"/>
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

<reference anchor="re2" target="https://github.com/google/re2/wiki/Syntax">
    <front>
        <title>RE2: a principled approach to regular expression matching</title>
        <author>
            <organization>Google</organization>
        </author>
        <date year="2024"/>
    </front>
</reference>

<reference anchor="UNICODE" target="https://www.unicode.org/versions/latest/">
    <front>
        <title>The Unicode Standard</title>
        <author>
            <organization>The Unicode Consortium</organization>
        </author>
        <date year="2024"/>
    </front>
</reference>

{backmatter}
