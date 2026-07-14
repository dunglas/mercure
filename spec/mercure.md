%%%
title = "The Mercure Protocol"
abbrev = "Mercure"
ipr = "trust200902"
area = "Web and Internet Transport"
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
It pushes any web content to web browsers and other clients over a single long-lived HTTP
connection, avoiding polling and its associated latency and power cost. Mercure is especially
useful for delivering real-time updates of
resources served through sites and web APIs to web and mobile applications, and can also
be used as a general-purpose publish-subscribe system.

Subscription requests are relayed through hubs, which validate them.
When new or updated content becomes available, hubs check whether subscribers are authorized
to receive it and then distribute it.

{mainmatter}

# Introduction

Mercure is a protocol for pushing updates of web resources to clients over HTTP. It builds on
Server-Sent Events [@!HTML] for delivery and on JSON Web Signatures
[@!RFC7515] for authorization, so that it can be implemented on top of existing HTTP
infrastructure and consumed natively by web browsers.

Publishers send updates to a hub. Subscribers open a long-lived HTTP connection to the hub and
declare, using topic matchers, which topics they want to receive. The hub checks authorization
and dispatches matching updates, including updates marked as private that only authorized
subscribers may receive.

This document specifies the subscription and publication interfaces, the topic matcher types,
the OAuth 2.0-based authorization model, reconnection and state reconciliation,
active-subscription tracking, discovery, and update encryption.

Some normative references of this document are dated snapshots (Review Drafts) of WHATWG Living
Standards. Conformance is evaluated against the cited snapshots; later changes to those
standards do not automatically apply to this protocol.

# Terminology

The key words **MUST**, **MUST NOT**, **REQUIRED**, **SHALL**, **SHALL NOT**, **SHOULD**,
**SHOULD NOT**, **RECOMMENDED**, **NOT RECOMMENDED**, **MAY**, and **OPTIONAL** in this document
are to be interpreted as described in BCP 14 [@!RFC2119] [@!RFC8174] when, and only when, they
appear in all capitals, as shown here.

*   Topic: The unit to which one can subscribe for changes. The topic is identified by a string
    that can be an IRI [@!RFC3987]. Topic strings **MUST** be valid UTF-8 [@!RFC3629] and
    **MUST NOT** contain C0 (U+0000–U+001F) or C1 (U+0080–U+009F) control characters, U+007F
    (DEL), or Unicode format characters (general category `Cf` [@!UNICODE], such as the
    bidirectional and zero-width controls), which are invisible and enable identifier spoofing.
    Character classifications are those of the Unicode version cited in [@!UNICODE]; hubs
    **MAY** also reject characters that later Unicode versions assign to general category `Cf`.
    An update is about exactly one topic.
*   Update: The message containing the updated version of the topic. An update can be marked as
    private; in that case, it **MUST** be dispatched only to subscribers allowed to receive it.
*   Topic matcher: An expression matched against one or more topics,
    depending on the matcher type.
*   Topic matcher type: The kind of a matching expression, which determines how the expression is
    interpreted. This document defines two matcher types, `exact` and `urlpattern`; others can be
    registered (see (#matcher-types)).
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
[@!HTML]. The `GET` HTTP method **MUST** be used. The connection
**SHOULD** use HTTP version 2 or higher to leverage multiplexing and other performance-related
features.

The subscriber specifies the topics to receive updates from using topic matcher query
parameters. The parameter name encodes the matcher type: the bare `match` parameter selects the
default `exact` matcher type, and a `match_<matcher-type>` parameter selects the named matcher
type — for example, `match_urlpattern` selects the `urlpattern` matcher type, and `match_exact` is
the explicit spelling of the default. The `<matcher-type>` suffix **MUST** be the matcher type
name in its canonical form as defined in (#matcher-types). A request **MAY** contain several such
parameters, in any combination. See (#matcher-types). These parameters select which topics the
subscriber receives; they do not by themselves grant access to private updates, which is
governed by the access token (see (#authorization)).

This mirrors the topic matcher list of authorization details (see (#topic-matcher-list)), where
the `match_type` member is optional and defaults to `exact`: omitting it there is equivalent to
using the bare `match` parameter here.

The query component of the subscription URL **MUST** be parsed into name/value pairs using the
`application/x-www-form-urlencoded` parsing algorithm of [@!URL] (the algorithm implemented by
`URLSearchParams` and used by `EventSource`). The reserved-namespace rule and the value
constraints below apply to the percent-decoded parameter names and values. A parameter name
given without a value is equivalent to that name with an empty value. Clients **MUST**
percent-encode any character in a matcher name or value that `application/x-www-form-urlencoded`
serialization would encode — notably `&`, `=`, `+`, `;`, and `%` — as `URLSearchParams`
does; this keeps parsing unambiguous across implementations (some form-urlencoded parsers treat
a raw `;` as a delimiter or reject a stray `%`).

The names of topic matcher query parameters are case-sensitive. A request using a parameter name
in the reserved `match` namespace (a name equal to `match`, or beginning with `match` under an
ASCII case-insensitive comparison) that does not correspond to a matcher type supported by the
hub (see (#matcher-types)) **MUST** be rejected with a 400 "Bad Request" HTTP status code. This
deliberately reserves the whole `match` prefix: unrelated query parameters whose names begin
with `match` cannot be used on the subscription URL, and a misspelled matcher type — or a
registered matcher type the hub does not implement — fails loudly instead of being silently
ignored.

The value of each topic matcher query parameter **MUST** be valid UTF-8 [@!RFC3629] and
**MUST NOT** contain C0 (U+0000–U+001F) or C1 (U+0080–U+009F) control characters, U+007F, or
Unicode format characters (general category `Cf` [@!UNICODE]).
A parameter value that is not valid for its matcher type (for example, a `match_urlpattern`
value that is not a well-formed URL Pattern) is equally invalid. Requests violating any of these
constraints **MUST** be rejected with a 400 "Bad Request" HTTP status code.

The subscriber receives updates for all topics matching at least one topic matcher according to
the matcher type rules.

To mitigate resource exhaustion, hubs **SHOULD** apply implementation-defined maximums to the
number of topic matcher query parameters in a single request and to the length of each
matcher's pattern. Requests exceeding any such limit **MUST** be rejected with a 400 "Bad
Request" HTTP status code. A subscription is created for every topic matcher query parameter
present in the request. Hubs **MAY** deduplicate subscriptions that have identical matcher type
and pattern. See (#subscription-events).

Because subscription connections are long-lived, hubs **SHOULD** also apply
implementation-defined limits to the number of concurrent connections held by a single client
(for example, per source address or per token subject) and in total, and **MAY** reject further
connection attempts with a 429 "Too Many Requests" HTTP status code [@!RFC6585].

The `EventSource` JavaScript interface [@!HTML] **MAY** be used to establish
the connection. Any other appropriate mechanism, including but not limited to readable streams
[@streams] and XMLHttpRequest [@xhr] (used by popular polyfills),
**MAY** also be used.

Web browsers enforce the CORS protocol [@!FETCH] on cross-origin `EventSource` connections.
Hubs serving browser-based subscribers on other origins **MUST** send the appropriate CORS
response headers. When the connection carries credentials (such as the cookie defined in
(#cookie)), the `Access-Control-Allow-Origin` response header **MUST NOT** be the `*` wildcard
and **MUST NOT** be reflected from arbitrary request origins: it **MUST** be restricted to an
explicit allowlist of trusted origins, and the hub **MUST** also send
`Access-Control-Allow-Credentials: true`. Reflecting arbitrary origins on a credentialed
endpoint would allow any site visited by the subscriber to read updates using the
subscriber's cookie.

The hub sends updates to the subscriber for topics matching the provided topic matchers.

If an update is marked as `private`, the hub **MUST NOT** dispatch it to subscribers not authorized
to receive it. See (#authorization).

The hub **MUST** send these updates as `text/event-stream`-compliant events
[@!HTML].

Event streams are long-lived responses and interact poorly with intermediaries that buffer
responses or terminate idle connections. When no update has been dispatched for an
implementation-defined period, hubs **SHOULD** send an SSE comment line (a line starting with
`:` [@!HTML]) as a keep-alive, and deployments **SHOULD** configure intermediaries not to
buffer event streams.

The `data` property **MUST** contain the topic's new version. It **MAY** be the full resource or
a partial update in formats such as JSON Patch [@RFC6902] or JSON Merge Patch [@RFC7396].

All other properties defined in the Server-Sent Events specification **MAY** be used and **MUST**
be supported by hubs.

The resource **MAY** be represented in a format with hypermedia capabilities such as
JSON-LD [@W3C.REC-json-ld11-20200716], Atom [@RFC4287], XML [@W3C.REC-xml-20081126] or HTML
[@!HTML].

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
url.searchParams.append('match', 'https://example.com/foo');
url.searchParams.append('match', 'bar');
url.searchParams.append('match_urlpattern', 'https://example.com/bar/:id');

const eventSource = new EventSource(url);

// The callback will be called every time an update is published
eventSource.onmessage = function ({data}) {
    console.log(data);
};
~~~

The hub **MAY** apply extra authorization rules not defined in this specification. See
(#authorization).

# Matcher Types

A topic matcher is an expression matched against topics; its matcher type determines how the
expression is interpreted. This document defines two matcher types, `exact` and `urlpattern`.
Hubs **MUST** support both.

Additional matcher types can be defined by other specifications and registered in the "Mercure
Topic Matcher Types" registry (see (#iana-considerations)). Hubs **MAY** support registered
additional matcher types and **SHOULD** advertise the complete set of matcher types they
support with the `mercure_matcher_types_supported` metadata member (see
(#protected-resource-metadata)). Requests and tokens using a matcher type the hub does not
support are rejected as defined in (#subscription) and (#topic-matcher-list): matcher
evaluation determines both routing and authorization, so a matcher the hub cannot interpret
must fail loudly rather than be skipped.

The matcher value `*` is reserved as a wildcard that matches every topic. It is recognized before
the matcher type is resolved, so it has this meaning regardless of matcher type and regardless of
whether `match_type` is supplied or defaulted (see (#topic-matcher-list)). As a consequence, a topic
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

The matcher type name is `exact`. It is the default matcher type: the corresponding subscribe
query parameter is the bare `match` (or, explicitly, `match_exact`), and it is the default
`match_type` value in authorization details (see (#topic-matcher-list)).

## URL Pattern

The hub **MUST** support using URL patterns [@!urlpattern] as matchers.

URL patterns **MAY** be absolute (e.g., `https://example.com/books/:id`) or relative
(e.g., `/.well-known/mercure/subscriptions/exact/:topic/:subscriber`). When evaluating
a relative pattern or a relative topic, the hub **MUST** use the hub's URL as the
base URL. This allows subscribers to match relative topics published by the hub
itself, such as subscription events (see (#subscription-events)).

URL patterns are evaluated per the URL Pattern Living Standard [@!urlpattern]; hubs **MUST NOT**
enable the `ignoreCase` option. Host components remain case-insensitive as defined by URL
canonicalization [@!RFC3986]; all other components are case-sensitive.

A topic that cannot be parsed as a URL reference against the hub's URL cannot be matched by a
URL pattern: hubs **MUST** treat its evaluation against any URL pattern as not matching.

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

The matcher type name is `urlpattern`. The corresponding subscribe query parameter is
`match_urlpattern`, and the corresponding `match_type` value in authorization details (see
(#topic-matcher-list)) is `urlpattern`.

## Summary of Matcher Types

| Matcher Type   | Subscribe Query Parameter     | `match_type` | Requirement  |
|----------------|-------------------------------|-------------|--------------|
| `exact`        | `match` (or `match_exact`)     | `exact`     | **MUST**     |
| `urlpattern`   | `match_urlpattern`             | `urlpattern`| **MUST**     |

This table lists the matcher types defined by this document; the "Mercure Topic Matcher Types"
registry (see (#iana-considerations)) records additional registered types.

# Publication

The publisher sends updates by issuing `POST` HTTPS requests to the hub URL. When it receives an
update, the hub dispatches it to subscribers using the established server-sent events connections.

The hub **MAY** also dispatch the update using other protocols such as WebSub
[@W3C.REC-websub-20180123] or ActivityPub [@W3C.REC-activitypub-20180123].

An application **MAY** deliver events directly to subscribers without an external hub. In that
case, the publish endpoint described in this section is not required.

The request **MUST** be encoded using the `application/x-www-form-urlencoded` format
[@!URL]. Field names and values **MUST** be UTF-8 [@!RFC3629]. The request
**MUST** contain exactly one `topic` field; all other fields defined below are optional:

*   `topic` (required): The identifier of the updated topic, and the resource against which private-read
    authorization is evaluated (see (#subscribers)). It is **RECOMMENDED** to use an IRI as
    identifier. This field **MUST** appear exactly once; a request carrying more than one `topic`
    field **MUST** be rejected with a 400 "Bad Request" HTTP status code. The topic value
    **MUST** conform to the constraints defined in (#terminology). The topic **MUST NOT** address
    the reserved hub namespace. To test this, the topic **MUST** be resolved against the hub's URL
    (see (#discovery)) using the URL parser of [@!URL] — the same algorithm and canonicalization
    used for URL Pattern matching (see (#url-pattern)). A topic addresses the reserved namespace
    when the resolved path component equals the path of the hub's URL or begins with that path
    followed by `/` — for the default hub URL (see (#discovery)), `/.well-known/mercure` and
    `/.well-known/mercure/...` — regardless of scheme or authority. Before this comparison, the path
    **MUST** have its dot-segments removed and its percent-encoded octets that correspond to
    unreserved characters decoded [@!RFC3986]; otherwise, variants such as
    `/.well-known/%6Dercure/...` or `/.well-known/mercure/../mercure/...` would bypass the check.
    A topic that cannot be parsed as a URL reference against the hub's URL does not address the
    reserved namespace (it cannot name a hub path) and is not rejected by this rule. This namespace is
    reserved for resources generated by the hub itself, including subscription events (see
    (#subscription-events)). Hubs **MUST** reject publish requests violating this rule with a 403
    HTTP status code. Checking the resolved path component (rather than a leading-substring match
    on the raw value) prevents a publisher from forging subscription events with an absolute topic
    such as `https://hub.example.com/.well-known/mercure/subscriptions/...`.
*   `data` (optional): the content of the new version of this topic. The value **MUST** be
    valid UTF-8 [@!RFC3629]. When dispatching the update, the hub **MUST** serialize the value as
    one SSE `data:` field per line, splitting on CR, LF, or CRLF, per the Server-Sent Events
    serialization rules [@!HTML]. A receiver reassembles the fields joined by LF, so a value
    containing CR or CRLF is received with those sequences normalized to LF; publishers that
    require byte-exact round-tripping (for example, of encrypted payloads) **SHOULD** encode the
    value (for example, with base64) before publication.
*   `private` (optional): if this field is present, the update **MUST NOT** be dispatched to
    subscribers not authorized to receive it. See (#authorization). The presence of the field
    name marks the update as private regardless of its value, whether or not a value is
    supplied; hubs **MUST NOT** interpret the field's value to determine privacy. It is
    **RECOMMENDED** to set the value to `on` for interoperability, but it **MAY** contain any
    value, including an empty string.
*   `id` (optional): the topic's revision identifier; used as the SSE `id` property.
    The provided ID **MUST NOT** start with the `#` character, **MUST NOT** be the reserved
    value `earliest` (see (#reconciliation)), and **MUST NOT** contain control characters
    (C0 (U+0000–U+001F), U+007F, or C1 (U+0080–U+009F)) or Unicode format characters (general
    category `Cf` [@!UNICODE]) — the same constraint as topics (see (#terminology)), since the
    ID also travels in the `Last-Event-ID` HTTP field.
    The provided ID **MAY** be a valid IRI. If omitted, the
    hub **MUST** generate either a valid IRI [@!RFC3987] or a relative reference consisting of a
    fragment (starting with `#`). A UUID [@RFC9562] or a DID [@DID] **MAY** be used as the IRI; a
    fragment is convenient to return an offset or a sequence that is unique for this hub. The hub
    **MAY** ignore the
    client-supplied ID and generate its own. The hub **MUST** reject client-supplied IDs
    violating the character constraints above with a 400 HTTP status code.
*   `type` (optional): the SSE `event` property (a specific event type). The value **MUST NOT**
    contain control characters (C0 (U+0000–U+001F), U+007F, or C1 (U+0080–U+009F)) or Unicode
    format characters (general category `Cf` [@!UNICODE]); hubs **MUST** reject violating values
    with a 400 HTTP status code.
*   `retry` (optional): the SSE `retry` property (the reconnection time). The value **MUST**
    consist solely of ASCII digits (U+0030–U+0039); hubs **MUST** reject violating values with
    a 400 HTTP status code.

To allow future extensions, hubs **MUST** ignore fields they do not recognize.

On success, the hub **MUST** return a 2xx HTTP status code, and the response body **MUST** be
the `id` generated by the hub for the update, served with the `text/plain` media type and the
UTF-8 charset (`Content-Type: text/plain; charset=utf-8`). Hubs **SHOULD** use 200 (OK). The status code 201
(Created) is **NOT RECOMMENDED**: an update is an ephemeral message rather than a resource
retrievable at a dereferenceable URL, so the `id` is an event cursor (see (#reconciliation)),
not a `Location`. The publisher **MUST** be authorized to publish updates; see (#authorization).

Hubs **SHOULD** apply implementation-defined maximums to the size of the request body and to
the length of individual fields. Requests exceeding any such limit **MUST** be rejected with a
413 "Content Too Large" HTTP status code.

Example:

~~~ http
POST /.well-known/mercure HTTP/1.1
Host: example.com
Content-Type: application/x-www-form-urlencoded
Authorization: Bearer [snip]

topic=https://example.com/foo&data=the%20content

HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8

urn:uuid:e1ee88e2-532a-4d6f-ba70-f0f8bd584022
~~~

# Authorization

The hub is an OAuth 2.0 protected resource [@!RFC6749]. To prove that they are authorized,
publishers **MUST** present an access token to the hub (except in the closed-network
deployments described in the note below), and subscribers **MUST** present
an access token to receive updates marked as private. Hubs **MAY** accept unauthenticated
subscribers; such subscribers receive only updates that are not marked as private. Hubs
**MAY** instead require all subscribers to present an access token, according to their own
policy. The access token
**MUST** be a JWT [@!RFC7519] structured as a JWT access token [@!RFC9068] — in particular
using the `at+jwt` media type — carried as a JWS [@!RFC7515] in compact serialization, and
**MUST** be validated as described in (#token-validation). Hubs apply the [@!RFC9068]
validation rules with the deviations for self-issued tokens defined there. The token
**SHOULD** be short-lived, especially when the subscriber is a web browser.

Access tokens **MAY** be issued by an OAuth 2.0 authorization server or self-issued by the
publisher (for example, signed with a key shared out of band with the hub). The hub need not
operate or trust an external authorization server. When an authorization server is used, the hub
**MAY** advertise it through protected resource metadata (see (#discovery)). Different keys
**SHOULD** be used to sign subscribers' and publishers' tokens so that compromise of one role
does not entail compromise of the other.

For example, a minimal self-issued subscriber token is a JWS with the protected header
`{"alg": "ES256", "typ": "at+jwt"}` and the following claims (see (#token-validation) for the
claims a hub requires):

~~~ json
{
  "aud": "https://hub.example.com/.well-known/mercure",
  "exp": 1767225600,
  "sub": "urn:uuid:bb3de268-05b0-4c65-b44e-8f9acefc29d6",
  "authorization_details": [
    {
      "type": "mercure",
      "actions": ["subscribe"],
      "topics": [{"match": "https://example.com/books/:id", "match_type": "urlpattern"}]
    }
  ]
}
~~~

Authorization is expressed with the `authorization_details` claim [@!RFC9396]; see
(#authorization-details). Routing (which topics a subscriber listens to, via the query
parameters of (#subscription)) is independent of authorization (what a token permits): the query
parameters never grant access to private updates.

Note: Hubs **MAY** also be deployed without requiring authorization for publication (for
example, on a trusted private network). Because any client able to reach such a hub can
publish at will, these deployments **MUST NOT** be reachable from networks containing
untrusted clients. The remainder of this section assumes token-based authorization is in use
for publication.

## Presenting the Access Token

Three mechanisms are defined to present the access token to the hub, following the OAuth 2.0
Bearer Token Usage specification [@!RFC6750] where applicable:

*   an `Authorization` HTTP header with the `Bearer` scheme [@!RFC6750],
*   a cookie (a Mercure extension for web browsers, see below),
*   an `access_token` URI query parameter [@!RFC6750].

When any of these mechanisms is used, the connection **MUST** use an encryption layer such as
HTTPS.

Per [@!RFC6750], clients **MUST NOT** use more than one mechanism to transmit the token in a
single request. When more than one mechanism is nevertheless present, the hub **MUST** select
exactly one token using the
following precedence, from highest to lowest: the `Authorization` HTTP header, then the
`access_token` query parameter, then the cookie. The token from the selected mechanism **MUST**
be used and the others **MUST** be ignored. Concretely: if an `Authorization` HTTP header is
present, its token **MUST** be used and the `access_token` query parameter and the cookie **MUST**
be ignored; otherwise, if an `access_token` query parameter is present, it **MUST** be used and the
cookie **MUST** be ignored; otherwise, the cookie, if any, **MUST** be used.

### Authorization HTTP Header

If the publisher or the subscriber is not a web browser, it **SHOULD** use an `Authorization`
HTTP header. This header **MUST** contain the string `Bearer` followed by a space character and
by the access token, as defined in [@!RFC6750]. As with every HTTP authentication scheme, the
scheme name is matched case-insensitively [@RFC9110].

### Cookie

Per the `EventSource` specification [@!HTML], web browsers cannot set
custom HTTP headers on such connections, and the connections can only be established using the
`GET` HTTP method. However, cookies are supported and can be included even in cross-domain
requests when the CORS credentials mode is enabled through the `withCredentials` attribute of
`EventSource` [@!HTML].
This cookie mechanism is a Mercure-specific extension to [@!RFC6750]; hubs that support it
**SHOULD** advertise it in their protected resource metadata (see (#discovery)).

If the publisher or the subscriber is a web browser, it **SHOULD**, whenever possible, send a
cookie containing the access token when connecting to the hub. It is **RECOMMENDED** to name the
cookie `__Secure-mercure_access_token`: the `__Secure-` name prefix
[@!I-D.ietf-httpbis-rfc6265bis] makes user agents refuse the cookie over insecure transport
while — unlike the `__Host-` prefix — remaining compatible with the `Domain` attribute used
below. A different name **MAY** be used to prevent conflicts when several hubs share the same
domain.

The cookie **SHOULD** be set during discovery (see (#discovery)) to improve overall security.
Consequently, if the cookie is set during discovery, the publisher and the hub **MUST** share
the same registrable domain (eTLD+1). The `Domain` attribute **MAY** be used to allow the
publisher and the hub to use different subdomains of that registrable domain. See (#discovery).

The cookie **MUST** have the `Secure` and `HttpOnly` attributes set [@!RFC6265]. The cookie
**SHOULD** also have `SameSite=Strict` [@!I-D.ietf-httpbis-rfc6265bis]; `SameSite=Lax` **MAY**
be used if cross-site discovery flows require it.
The cookie's `Path` attribute **SHOULD** be set to the path of the hub's subscription URL. See
(#security-considerations).

### URI Query Parameter

If the client cannot use an `Authorization` HTTP header or a cookie, the access token **MAY** be
passed in the `access_token` URI query parameter as defined in [@!RFC6750] section 2.3.

For example, the client makes the following HTTP request using transport-layer security:

~~~ http
GET /.well-known/mercure?match=https%3A%2F%2Fexample.com%2Fbooks%2Ffoo&access_token=<token> HTTP/1.1
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
section 3. For the token-related failures below (missing, invalid, or insufficiently-scoped
token), the `error` code is carried as the `error` attribute (`auth-param`) of a
`WWW-Authenticate: Bearer` challenge, not as a response body field; every such 401 or 403 carries
that header [@!RFC9110], and the challenge **SHOULD** include the `resource_metadata` parameter (see
(#discovery)) per [@!RFC9728], not only when no token is presented. A 400 for a request that is
malformed independently of the access token (for example, a missing `topic` field or an unknown
matcher query parameter) need not carry a Bearer challenge:

*   If no access token is presented and the requested operation requires one (see above), the
    hub **MUST** return a 401 "Unauthorized" status code
    with a `WWW-Authenticate: Bearer` challenge and **MUST NOT** include an error code. The
    challenge **SHOULD** include a `resource_metadata` parameter pointing to the hub's protected
    resource metadata (see (#discovery)) per [@!RFC9728].
*   If a token is presented but fails validation as defined in (#token-validation), or carries
    `mercure` authorization details that fail to parse or validate or that exceed
    implementation-defined limits (see (#authorization-details)), the hub
    **MUST** return a 401 status code with `error="invalid_token"`.
*   If a valid token does not authorize the requested operation, the hub **MUST** return a 403
    "Forbidden" status code with `error="insufficient_scope"`.
*   If the request is malformed, the hub **MUST** return a 400 "Bad Request" status code. When
    the malformed element is the access token or its presentation, the response **MUST** carry
    a `WWW-Authenticate: Bearer` challenge with `error="invalid_request"` [@!RFC6750]. A 400
    for a request malformed independently of the access token conveys no [@!RFC6750] error code
    unless the hub chooses to include such a challenge, since these codes are defined only as
    challenge parameters.

Returning `invalid_token` for every presented-token failure (rather than distinguishing a bad
signature from an expired or not-yet-valid token) avoids disclosing why validation failed.

For error responses that are not conveyed through the `WWW-Authenticate: Bearer` challenge (for
example, a 400 for a request malformed independently of the access token, a 429, or the 403 for
a publish targeting the reserved namespace), the hub **MAY** return a problem details document
[@RFC9457] as the response body. The OAuth error codes above remain carried in the challenge and
are not duplicated in such a body.

## Token Validation {#token-validation}

Hubs **MUST** validate access tokens as JWT access tokens [@!RFC9068] and in accordance with the
JSON Web Token Best Current Practices [@!RFC8725]. The requirements below profile those
documents for hubs rather than replace them; where self-issued tokens deviate from [@!RFC9068],
the deviation is stated explicitly. In particular:

*   Hubs **MUST** verify that the token header `typ` is `at+jwt`, or the equivalent
    `application/at+jwt`, compared with the `application/` prefix omitted and case-insensitively
    per [@!RFC7515] section 4.1.9 [@!RFC9068], so that tokens issued for other purposes (for
    example, OpenID Connect ID Tokens) are not accepted.
*   Hubs **MUST** be configured with an explicit allowlist of accepted signature algorithms and
    **MUST** reject any token whose `alg` is not on that allowlist. Hubs **MUST NOT** accept
    `alg=none`, **MUST NOT** derive the set of acceptable algorithms from the token, and **MUST**
    verify that `alg` is compatible with the key used for verification (preventing
    algorithm-confusion attacks). The allowlist **SHOULD** include at minimum `EdDSA`, `ES256`,
    and `RS256`, and **SHOULD NOT** include algorithms known to be cryptographically weak at
    the time of deployment.
*   Hubs **MUST** select the verification key from a preconfigured or pre-trusted set and
    **MUST NOT** use key material supplied by the token itself (such as the `jwk`, `jku`, or
    `x5u` header parameters). When more than one trusted key is in use — for example, separate
    publisher and subscriber keys or rotated keys — the `kid` header parameter **MAY** be used
    as a hint to choose among the trusted keys, as **MAY** the role of the endpoint; the token
    can thus influence which trusted key is tried, but never introduce a new one.
    Trusted keys are obtained from static configuration, from the authorization server's JWK
    Set [@!RFC7517] discovered through its metadata [@!RFC8414], or from the hub's protected
    resource metadata (see (#discovery)). This specification defines no hub-specific key
    distribution endpoint. When the hub trusts key material from more than one issuer, it
    **MUST** verify the signature using only the key(s) associated with the token's `iss` (for
    self-issued tokens, the key(s) bound to the out-of-band trust relationship), never a pooled
    set spanning issuers, so that a token signed by one trusted issuer cannot be accepted under
    another issuer's identity.
*   Hubs **MUST** enforce the `exp` claim [@!RFC7519], including on the first request received
    bearing a token, and **MUST** reject a token that has no `exp` claim ([@!RFC9068] requires
    it). Hubs **MUST** enforce the `nbf` claim if present.
*   Hubs **MUST** be configured with their resource identifier and **MUST** verify that it
    appears in the token `aud` claim [@!RFC9068]. It is **RECOMMENDED** that the resource
    identifier be the canonical URL of the hub (for example,
    `https://hub.example.com/.well-known/mercure`), which gives deployments an obvious default. Per [@!RFC7519], `aud` **MAY** be a single
    string or an array of strings; the resource identifier matches when it equals that string or
    is a member of that array. This bounds a token to its intended hub and mitigates replay across
    hubs that share signing keys (see (#hub-trust)). When the hub trusts one or more
    authorization servers, it **MUST** verify that the `iss` claim is the issuer identifier of
    one of them [@!RFC9068], whether or not it advertises them in its protected resource
    metadata (see (#discovery)). A hub that accepts only self-issued tokens and trusts no
    authorization server **MAY** omit the `iss` check; such a hub **MUST** use an `aud` value
    unique to it (see (#hub-trust)) so that the audience alone bounds the token to the hub.

[@!RFC9068] requires authorization servers to populate the `iss`, `exp`, `aud`, `sub`,
`client_id`, `iat`, and `jti` claims. Hubs **MUST** enforce `exp`, `aud`, and — when an
authorization server is advertised — `iss`, as described above. Hubs accepting self-issued
tokens **SHOULD NOT** require the remaining [@!RFC9068] claims unless they rely on them: a
minimal self-issued token carries only `exp`, `aud`, the `authorization_details` claim (see
(#authorization-details)), and `sub` when subscription events are used (see
(#subscription-events)).

The `sub` claim, when present, identifies the subscriber and is used to derive subscription
event identifiers (see (#subscription-events)).

Failure of any of these checks **MUST** be reported as defined in (#error-responses).

## Authorization Details {#authorization-details}

Authorization is expressed with the `authorization_details` claim [@!RFC9396], a JSON array of
authorization detail objects. This specification defines the authorization detail type `mercure`.
[@!RFC9396] does not establish a registry of authorization details type identifiers (it leaves
their registration with authorization servers out of scope); the `mercure` type is defined by
this document. Authorization servers supporting it advertise the value `mercure` in their
`authorization_details_types_supported` metadata [@!RFC9396].

A `mercure` authorization detail object:

*   **MUST** have a `type` property whose value is the string `mercure`.
*   **MUST** have an `actions` property: a non-empty JSON array of strings (RFC 9396 `actions`
    field). This document defines the actions `publish` and `subscribe`; additional actions can
    be registered in the "Mercure Actions" registry (see (#iana-considerations)). Hubs **MUST**
    ignore action values they do not recognize: an unrecognized action grants nothing, and its
    presence does not invalidate the token. This lets issuers include actions defined by future
    specifications without breaking deployed hubs; a detail whose `actions` contains no action
    recognized by the hub simply grants nothing. A non-string entry in `actions` remains a
    validation failure.
*   **MUST** have a `topics` property: a non-empty JSON array of topic matcher objects (see
    (#topic-matcher-list)) identifying the topics the actions apply to.
*   **MAY** have a `payload` property (a JSON object), meaningful only for the `subscribe` action
    (see (#payloads)).

A token grants an action on a topic when it carries a `mercure` authorization detail whose
`actions` includes that action and one of whose `topics` matches the topic. Tokens with no
`mercure` authorization detail grant no publish or subscribe rights.

Hubs **SHOULD** apply implementation-defined maximums to the number of `mercure` authorization
details, to the number of entries in each `topics` array, and to the length of individual
patterns. Tokens exceeding any such limit **MUST** be rejected as invalid tokens, with a 401
status code and `error="invalid_token"` (see (#error-responses)). If any
`mercure` authorization detail fails to parse or validate (including failures specific to a
matcher's `match_type`), the hub **MUST** reject the token the same way and **MUST
NOT** act on the basis of the remaining entries; partial acceptance is forbidden because it
would silently alter the effective authorization of the token. These are defects of the
presented token, not of the request, so they are reported as `invalid_token` rather than
`invalid_request` [@!RFC6750].

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

When the subscriber presented an access token, the hub **MUST** close the connection no
later than the token's `exp` time, since `exp` is required (see (#token-validation)).
Since `exp` alone cannot revoke an already-established
long-lived connection, hubs **SHOULD** also impose a maximum connection lifetime independent of
`exp` and close connections that exceed it, requiring the subscriber to reconnect and
re-authenticate.

For example, a subscriber may listen to all books via the routing matcher
`match_urlpattern=https://example.com/books/:id` while its token authorizes reading only specific
books:

~~~ json
{
  "authorization_details": [
    {
      "type": "mercure",
      "actions": ["subscribe"],
      "topics": [
        {"match": "https://example.com/books/1", "match_type": "exact"},
        {"match": "https://example.com/books/7", "match_type": "exact"}
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
topic matcher itself, and **MAY** have an OPTIONAL `match_type` property containing the matcher
type. The value of `match_type` is case-sensitive and **MUST** be the name of a matcher type
supported by the hub (see (#matcher-types)); this document defines `exact` and `urlpattern`.
Unlike unrecognized actions (see (#authorization-details)), a `match_type` the hub does not
support **MUST** cause the token to be rejected: the grant cannot be evaluated, and skipping it
would silently alter the effective authorization of the token. If no `match_type` key is
present, the hub **MUST** assume the `exact`
matcher type. A `match` value of `*` is the reserved wildcard and matches every topic regardless
of `match_type`, including when `match_type` is absent (see (#matcher-types)).

Any entry that is not a JSON object, or that fails to parse or validate as a topic matcher,
**MUST** cause the token to be rejected with a 401 status code and `error="invalid_token"` as
defined in (#authorization-details).

## Payloads

User-defined data can be attached to a subscription and made available through the subscription
API and in subscription events. See (#subscription-events).

A `mercure` authorization detail with the `subscribe` action **MAY** carry a `payload` JSON
object. The `payload` of the first `subscribe` authorization detail whose `topics` matches the
subscription's own matcher (the `match` or `match_<matcher-type>` query parameter value) **MUST** be
included under the `payload` key of the JSON object describing the subscription, both in the
subscription API and in subscription events. A `subscribe` detail whose `topics` contains the `*`
wildcard matches every subscription and can serve as a default.

Matching here treats the subscription's own matcher string as if it were a topic: for example,
the `urlpattern` matcher `https://example.com/bar/:id` in a `subscribe` detail matches the
subscription created by `match_urlpattern=https://example.com/bar/:id`, and an `exact` matcher
matches a subscription whose matcher string is byte-for-byte identical to it.

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
            "topics": [{"match": "https://example.com/bar/:id", "match_type": "urlpattern"}],
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
implement an event store [@EventSourcing].

To allow re-establishment in case of connection loss, events dispatched by the hub **MUST**
include an `id` property. The value of this property **SHOULD** be an IRI [@!RFC3987]. A UUID
[@RFC9562] or a DID [@DID] **MAY** be used.

Per the server-sent events specification, the subscriber tries to reconnect automatically in
case of connection loss. During reconnection, the subscriber **MUST** send the last received
event ID in a `Last-Event-ID` HTTP request header [@!HTML].

To fetch any update dispatched between the initial resource generation by the publisher and the
connection to the hub, the subscriber **MUST** send the event ID provided during discovery
either as a `Last-Event-ID` header or as a `last_event_id` query parameter. See (#discovery).

`EventSource` implementations may not allow setting HTTP headers on the first connection (before
a reconnection), and web browser implementations do not allow it.

To work around this, the hub **MUST** also accept the last event ID in a query parameter named
`last_event_id`.

If both the `Last-Event-ID` HTTP header and the `last_event_id` query parameter are present, the
HTTP header **MUST** take precedence.

If the `Last-Event-ID` HTTP header or the `last_event_id` query parameter is present, the hub
**SHOULD** send all events published after the one bearing this identifier to the subscriber,
subject to authorization.

The authorization rules defined in (#subscribers) apply to replayed events identically to live
events: the hub **MUST** re-evaluate each candidate replayed event against the current access
token before dispatching it. Events whose `private` flag is set and that the token does not
authorize the subscriber to read (see (#authorization-details)) **MUST NOT** be dispatched,
regardless of any authorization that may have applied at publication time.

The reserved value `earliest` requests that the hub send all updates it has for the subscribed
topics. The hub **MAY** ignore this request according to its own policy. Hub-generated
identifiers are IRIs or fragments (see above) and publishers are forbidden from supplying
`earliest` as an update ID (see (#publication)), so `earliest` cannot collide with an event
identifier.

The hub **MAY** discard some events for operational reasons. When the request contains a
`Last-Event-ID` HTTP header or a `last_event_id` query parameter, the hub **MUST** set a
`Mercure-Last-Event-ID` field on the HTTP response. This document defines the
`Mercure-Last-Event-ID` response field and registers it in (#iana-considerations) rather than
reusing `Last-Event-ID`, whose registration [@!HTML] defines request semantics only.

The value of this response field **MUST** be the identifier of the event preceding the first
event sent to the subscriber, or the reserved value `earliest` if there is no preceding event
(for example, when the hub history is empty, when the subscriber requests the earliest event,
or when the requested event does not exist or has been discarded).

Subscribers using the hub as an event store can use the returned identifier as a recovery
anchor; subscribers that only need to detect data loss can compare it against the requested
value (a different value indicates that loss may have occurred).

Note: Event identifiers are cursors, and the hub exposes them to subscribers, both through the
`last_event_id` value provided during discovery and through the `Mercure-Last-Event-ID` response
field.
The field value can be the identifier of an event immediately preceding one the subscriber is
not authorized to receive. A subscriber can therefore infer the existence of such an event and,
when the hub uses time-ordered identifiers (e.g., UUIDv7 [@!RFC9562]), its approximate timing
and ordering. Operators handling sensitive private updates **SHOULD** generate opaque, random
event identifiers (e.g., UUIDv4 [@!RFC9562]), or expose an encrypted form of an internal ordered
identifier, so that the cursor discloses nothing beyond what the subscriber already knows.

The subscriber **SHOULD NOT** assume that no events will be lost (events may be lost, for
instance, if the hub stores only a limited number of events in its history). In some cases (for
example, when sending partial updates in the JSON Patch [@RFC6902] format, or when using the
hub as an event store), lost updates can cause data loss.

To detect data loss, the subscriber **MAY** compare the value of the `Mercure-Last-Event-ID`
response field with the last event ID it requested. In case of data loss, the subscriber
**SHOULD** re-fetch the original topic.

Note: Native `EventSource` implementations do not expose HTTP response headers. However,
polyfills and server-sent events clients in most programming languages do.

The hub **MAY** also specify the reconnection time using the `retry` key, as defined by the
server-sent events format.

# Active Subscriptions

Mercure provides a mechanism to track active subscriptions. If the hub supports this optional set
of features, updates will be published when a subscription is created, or terminated, and a web API
exposes the list of active subscriptions. Hubs supporting this feature **SHOULD** advertise it
with the `mercure_subscriptions` metadata member (see (#protected-resource-metadata)).

Variables are templated and expanded following [@!RFC6570].

## Subscription Events

If the hub supports the active subscriptions feature, it **MUST** publish an update every time a
subscription is created or terminated.

The topic of these updates **MUST** be the path of the hub's URL followed by an expansion of
`/subscriptions/{match_type}/{match}/{subscriber}` — for the default hub URL,
`/.well-known/mercure/subscriptions/{match_type}/{match}/{subscriber}` — with the following
variables:

*   `{match_type}`: the topic matcher type used for this subscription. The value **MUST** be the
    matcher type name in its canonical case as defined in (#matcher-types) (e.g., `urlpattern`,
    `exact`). URL path components are case-sensitive.
*   `{match}`: the topic matcher used for this subscription
*   `{subscriber}`: a unique identifier for the subscriber. Subscribers and publishers
    **MUST NOT** forge, supply, or override this value through query parameters, headers,
    request bodies, or any other client-controlled channel. The hub **MUST** either generate
    the identifier itself (for example, a random UUID [@RFC9562]) or derive it from
    information it has cryptographically validated — typically the pair formed by the token's
    issuer and its `sub` claim (after validation per (#token-validation)). A derivation
    **MUST** incorporate the issuer (for self-issued tokens, the out-of-band trust
    relationship under which the signing key is accepted): `sub` alone is unique only within
    one issuer, and two trusted issuers can assign the same value to different
    subscribers. Hub-generated
    identifiers have a privacy advantage: they do not disclose the `sub` value to the other
    subscribers authorized for subscription events. The hub **MUST** ensure that the
    identifier is unique among active subscriptions.

Note: Because strings containing reserved characters (e.g., URIs, URL Patterns, and URI
Templates) can be used for the `{match}` and `{subscriber}` variables, per [@!RFC6570] the
value of each variable **MUST** be percent-encoded exactly once during expansion, encoding the
raw matcher or subscriber string as a whole (any `%` characters it already contains are
themselves encoded). Hubs **MUST NOT** double-encode an already-encoded value.

`{subscriber}` **SHOULD** be an IRI [@!RFC3987]. A UUID [@RFC9562] or a
DID [@DID] **MAY** also be used.

The content of the update **MUST** be a JSON-LD [@!W3C.REC-json-ld11-20200716] document containing
at least the following properties:

*   `@context`: the fixed value `https://mercure.rocks/`. `@context` **MAY** be omitted if
    already defined in a parent node. See (#json-ld-context).
*   `id`: the identifier of this update; **MUST** be the same value as the subscription update's
    topic.
*   `type`: the fixed value `Subscription`.
*   `match_type`: the topic matcher type used for this subscription. The value is case-sensitive
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
   "id": "/.well-known/mercure/subscriptions/urlpattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6",
   "type": "Subscription",
   "match_type": "urlpattern",
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

The web API **MUST** expose endpoints whose paths are the path of the hub's URL followed by
the patterns below (shown expanded for the default hub URL):

*   `/.well-known/mercure/subscriptions`: the collection of subscriptions.
*   `/.well-known/mercure/subscriptions/{match_type}/{match}`: the collection of subscriptions
    for the given topic matcher.
*   `/.well-known/mercure/subscriptions/{match_type}/{match}/{subscriber}`: a specific
    subscription.

To access these URLs, clients **MUST** be authorized according to the rules defined in
(#authorization). The topic to authorize is the requested URL in relative form: its absolute
path (for example, `/.well-known/mercure/subscriptions/{match_type}/{match}`), which is the
same form used for subscription event topics (see (#subscription-events)); any query component
of the request URL is not part of the topic. The hub **MUST**
verify that a `mercure` authorization detail in the access token grants the `subscribe` action
on this relative topic, evaluated with the matcher rules of (#matcher-types): `exact` matchers
are compared byte-for-byte against the relative form, while URL patterns — absolute, or
relative and then resolved against the hub's URL — are evaluated against it per
(#url-pattern). Because subscription event topics and subscription API URLs share this
canonical relative form, a single matcher covers both the events and the API resources
describing the same subscriptions. The same matching applies to every endpoint shape above: the
collection URL `/.well-known/mercure/subscriptions`, the per-matcher collection URL, and the
single-subscription URL are each matched as a topic, so a token whose `subscribe` matcher selects
a broader set (for example a `urlpattern` covering the subscriptions namespace, or the reserved
`*`) grants access to the corresponding endpoints. If no detail grants `subscribe` on the
requested URL, the hub **MUST** answer `403` as defined in (#authorization).

The web API **MUST** set the `Content-Type` HTTP header to `application/ld+json`.

URLs returning a single subscription (following the pattern
`/.well-known/mercure/subscriptions/{match_type}/{match}/{subscriber}`) **MUST** expose the same
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

In addition, all endpoints **MUST** set the `last_event_id` property at the root of the returned
JSON-LD document:

*   `last_event_id`: the identifier of the last event dispatched by the hub at the time of this
    request (see (#reconciliation)). The value **MUST** be `earliest` if no events have been
    dispatched yet. This value **SHOULD** be passed back to the hub when subscribing to
    subscription events to prevent data loss.

Active subscription collections can be large. Hubs **MAY** truncate or paginate collection
responses according to an implementation-defined policy; each returned document **MUST** remain
valid as described above. Pagination mechanisms are out of scope for this specification.

Because data returned by this web API is volatile, clients **SHOULD** validate that a cached
response is still fresh before using it.

Examples:

~~~ http
GET /.well-known/mercure/subscriptions HTTP/1.1
Host: example.com

HTTP/1.1 200 OK
Content-Type: application/ld+json
Link: <https://example.com/.well-known/mercure>; rel="mercure"
ETag: "urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb"
Cache-Control: must-revalidate

{
   "@context": "https://mercure.rocks/",
   "id": "/.well-known/mercure/subscriptions",
   "type": "Subscriptions",
   "last_event_id": "urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb",
   "subscriptions": [
      {
         "id": "/.well-known/mercure/subscriptions/urlpattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6",
         "type": "Subscription",
         "match_type": "urlpattern",
         "match": "https://example.com/:selector",
         "subscriber": "urn:uuid:bb3de268-05b0-4c65-b44e-8f9acefc29d6",
         "active": true,
         "payload": {"foo": "bar"}
      },
      {
         "id": "/.well-known/mercure/subscriptions/exact/https%3A%2F%2Fexample.com%2Fa-topic/urn%3Auuid%3A1e0cba4c-4bcd-44f0-ae8a-7b76f7ef1280",
         "type": "Subscription",
         "match": "https://example.com/a-topic",
         "match_type": "exact",
         "subscriber": "urn:uuid:1e0cba4c-4bcd-44f0-ae8a-7b76f7ef1280",
         "active": true,
         "payload": {"baz": "bat"}
      },
      {
         "id": "/.well-known/mercure/subscriptions/urlpattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Aa6c49794-5f74-4723-999c-3a7e33e51d49",
         "type": "Subscription",
         "match_type": "urlpattern",
         "match": "https://example.com/:selector",
         "subscriber": "urn:uuid:a6c49794-5f74-4723-999c-3a7e33e51d49",
         "active": true,
         "payload": {"foo": "bap"}
      }
   ]
}
~~~

~~~ http
GET /.well-known/mercure/subscriptions/urlpattern/https%3A%2F%2Fexample.com%2F%3Aselector HTTP/1.1
Host: example.com

HTTP/1.1 200 OK
Content-Type: application/ld+json
Link: <https://example.com/.well-known/mercure>; rel="mercure"
ETag: "urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb"
Cache-Control: must-revalidate

{
   "@context": "https://mercure.rocks/",
   "id": "/.well-known/mercure/subscriptions/urlpattern/https%3A%2F%2Fexample.com%2F%3Aselector",
   "type": "Subscriptions",
   "last_event_id": "urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb",
   "subscriptions": [
      {
         "id": "/.well-known/mercure/subscriptions/urlpattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6",
         "type": "Subscription",
         "match": "https://example.com/:selector",
         "match_type": "urlpattern",
         "subscriber": "urn:uuid:bb3de268-05b0-4c65-b44e-8f9acefc29d6",
         "active": true,
         "payload": {"foo": "bar"}
      },
      {
         "id": "/.well-known/mercure/subscriptions/urlpattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Aa6c49794-5f74-4723-999c-3a7e33e51d49",
         "type": "Subscription",
         "match": "https://example.com/:selector",
         "match_type": "urlpattern",
         "subscriber": "urn:uuid:a6c49794-5f74-4723-999c-3a7e33e51d49",
         "active": true,
         "payload": {"foo": "bap"}
      }
   ]
}
~~~

~~~ http
GET /.well-known/mercure/subscriptions/urlpattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6 HTTP/1.1
Host: example.com

HTTP/1.1 200 OK
Content-Type: application/ld+json
Link: <https://example.com/.well-known/mercure>; rel="mercure"
ETag: "urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb"
Cache-Control: must-revalidate

{
   "@context": "https://mercure.rocks/",
   "id": "/.well-known/mercure/subscriptions/urlpattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6",
   "type": "Subscription",
   "match": "https://example.com/:selector",
   "match_type": "urlpattern",
   "subscriber": "urn:uuid:bb3de268-05b0-4c65-b44e-8f9acefc29d6",
   "active": true,
   "payload": {"foo": "bar"},
   "last_event_id": "urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb"
}
~~~

# JSON-LD Context

Subscribers parsing JSON-LD documents produced by the hub **SHOULD NOT** automatically
dereference the `@context` URL. The context below is fixed and **MAY** be embedded in client
implementations or cached locally; this avoids both unnecessary network requests and the risk
that an attacker able to control responses from the context URL alters the semantics of
subsequently parsed documents. The context URL serves as a stable identifier: its normative
definition is the document below, embedded in this specification, and dereferencing the URL is
never required to implement or use the protocol.

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
   "match_type": "mercure:match_type",
   "match": "mercure:match",
   "subscriber": "mercure:subscriber",
   "active": "mercure:active",
   "payload": "mercure:payload",
   "last_event_id": "mercure:last_event_id"
}
}
~~~

# Discovery

## Hub Discovery

The discovery mechanism aims at identifying the URL of one or more hubs designated by the publisher.

The URL of the hub **SHOULD** be the "well-known" [@!RFC8615] fixed path `/.well-known/mercure`,
which gives publishers and subscribers a default requiring no configuration. The hub URL
advertised through discovery is authoritative: it **MAY** be any HTTPS URL, enabling
deployments behind path prefixes and several hubs sharing an origin. Protected resource
metadata (see (#protected-resource-metadata)) is derived from the hub URL per [@!RFC9728]
whatever its path, and the reserved namespace and subscription-event topics follow the hub
URL's path (see (#publication) and (#subscription-events)). Examples throughout this document
assume the default hub URL.

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
    parameter named `last_event_id`; this ensures that updates dispatched between the resource
    generation and the connection to the hub are not lost. See (#reconciliation).
*   `content-type`: the content type of the updates that will be pushed by the hub. If omitted,
    the subscriber **MUST** assume that the content type matches that of the original resource.
    The `content-type` attribute is especially useful to indicate that partial updates will be
    pushed, in formats such as JSON Patch [@RFC6902] or JSON Merge Patch [@RFC7396].

All these attributes are optional.

Minimal example:

~~~ http
GET /books/foo HTTP/1.1
Host: example.com

HTTP/1.1 200 OK
Content-Type: application/ld+json
Link: <https://example.com/.well-known/mercure>; rel="mercure"

{"@id": "/books/foo", "foo": "bar"}
~~~

## Protected Resource Metadata

As an OAuth 2.0 protected resource, the hub **SHOULD** publish OAuth 2.0 Protected Resource
Metadata [@!RFC9728] at the path derived from its URL as defined in that specification (for the
hub URL `/.well-known/mercure`, the metadata is served at
`/.well-known/oauth-protected-resource/.well-known/mercure`). Publishing the document is a
**SHOULD**; a document that is published **MUST** include the `resource` member (required by
[@!RFC9728]) and **MAY** include the others:

*   `resource` (required): the hub's resource identifier, which is the value clients **MUST**
    place in the access token `aud` claim (see (#token-validation)). Clients **MUST** verify that
    this value equals the resource identifier they used to derive the well-known URL [@!RFC9728].
*   `authorization_servers` (optional): issuer identifiers of the authorization servers that can
    issue tokens for the hub [@!RFC8414]. Omitted when tokens are self-issued.
*   `jwks_uri` (optional): the location of the hub's token verification keys, when not obtained
    from an authorization server.
*   `authorization_details_types_supported`: the authorization detail types the hub understands
    [@!RFC9728]; for hubs implementing this specification, the array contains `mercure` (see
    (#authorization-details)).
*   `bearer_methods_supported`: the [@!RFC6750] token presentation methods the hub actually
    accepts (see (#presenting-the-access-token)): `header`, plus `query` when the hub supports
    the URI query parameter method [@!RFC9728]. The cookie mechanism is not a [@!RFC6750]
    bearer method, so it is not listed here; it is advertised by the separate `mercure_cookie`
    member below.
*   `mercure_cookie` (optional): a boolean. When `true`, the hub also accepts the access token in
    a cookie (a Mercure extension to [@!RFC6750]; see (#cookie)). The cookie mechanism is
    advertised as a dedicated metadata member rather than a value of `bearer_methods_supported`,
    whose values are constrained to the [@!RFC6750] methods. This member is omitted when the hub
    does not offer cookie authorization.
*   `mercure_matcher_types_supported` (optional): a JSON array of strings listing the names of
    the topic matcher types the hub supports (see (#matcher-types)). When omitted, the
    supported set is exactly the two types defined by this document, `exact` and `urlpattern`;
    the member is only needed when the hub supports registered additional types.
*   `mercure_subscriptions` (optional): a boolean. When `true`, the hub implements the active
    subscriptions feature (see (#active-subscriptions)). This member is omitted when the hub
    does not implement it.

This protocol carries no version identifier: a future incompatible revision is expected to be
published as a new specification defining its own metadata members or well-known location.
Hubs implementing pre-standardization revisions of this protocol (which used a `topic`
subscribe query parameter and a bespoke token claim) do not publish protected resource
metadata; clients needing to coexist with them **MAY** treat the absence of this metadata as a
hint that the hub implements such a revision.

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
GET /books/foo HTTP/1.1
Host: example.com
Accept: application/ld+json

HTTP/1.1 200 OK
Content-Type: application/ld+json
Link: </books/foo.jsonld>; rel="self"
Link: <https://example.com/.well-known/mercure>; rel="mercure"

{"@id": "/books/foo", "foo": "bar"}
~~~

~~~ http
GET /books/foo HTTP/1.1
Host: example.com
Accept: text/html

HTTP/1.1 200 OK
Content-Type: text/html
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
GET /books/foo HTTP/1.1
Host: example.com
Accept: application/ld+json
Accept-Language: fr-FR

HTTP/1.1 200 OK
Content-Type: application/ld+json
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

Implementations **SHOULD** restrict JWE algorithms to those whose security properties remain
acceptable at the time of deployment; in particular, `RSA1_5` **MUST NOT** be used, due to
padding oracle vulnerabilities [@RFC8017].
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

# Using HTTP

This protocol follows the guidance of [@RFC9205]. It uses standard HTTP methods, standard status
codes, and registered media types, and does not overload their semantics. Clients discover a hub
from `rel=mercure` Web Linking [@!RFC8288] relations (see (#discovery)) rather than a mandated
well-known location: the fixed path `/.well-known/mercure` is only a **SHOULD** default, and the
authoritative hub URL advertised through discovery **MAY** be any HTTPS URL. The hub's own
resources (protected resource metadata and the subscription API) hang off that authoritative URL,
so the protocol adds no application semantics to a site-wide well-known space it does not own.

# IANA Considerations

## Well-Known URIs Registry

The "mercure" well-known URI described in (#discovery) is already registered in the
"Well-Known URIs" registry. IANA is requested to update the reference of the existing
registration to this document:

*   URI Suffix: mercure
*   Change Controller: IETF
*   Specification document(s): This specification, (#discovery)
*   Status: permanent
*   Related information: N/A

## Link Relation Types Registry

The "mercure" link relation type described in (#discovery) is already registered in the "Link
Relation Types" registry. IANA is requested to update the reference of the existing
registration to this document:

*   Relation Name: mercure
*   Description: The Mercure Hub to use to subscribe to updates of this resource.
*   Reference: This specification, (#discovery)

## OAuth Protected Resource Metadata Registry

The following values are to be registered in the "OAuth Protected Resource Metadata" registry
established by [@!RFC9728]:

*   Metadata Name: mercure_cookie
*   Metadata Description: Boolean indicating that the Mercure hub also accepts the access
    token in a cookie
*   Change Controller: IESG
*   Specification Document(s): This specification, (#protected-resource-metadata)

*   Metadata Name: mercure_matcher_types_supported
*   Metadata Description: JSON array listing the names of the Mercure topic matcher types
    supported by the hub
*   Change Controller: IESG
*   Specification Document(s): This specification, (#protected-resource-metadata)

*   Metadata Name: mercure_subscriptions
*   Metadata Description: Boolean indicating that the Mercure hub implements the active
    subscriptions feature
*   Change Controller: IESG
*   Specification Document(s): This specification, (#protected-resource-metadata)

## HTTP Field Name Registry

IANA is requested to register the following entry in the "Hypertext Transfer Protocol (HTTP)
Field Name Registry" defined by [@!RFC9110]:

*   Field Name: Mercure-Last-Event-ID
*   Status: permanent
*   Structured Type: N/A
*   Reference: This specification, (#reconciliation)
*   Comments: Response field carrying the identifier of the event preceding the first event
    sent, or the reserved value `earliest`

## Mercure Topic Matcher Types Registry

IANA is requested to establish a "Mercure Topic Matcher Types" registry. New registrations
follow the Specification Required policy [@!RFC8126].

A matcher type name **MUST** consist of lowercase ASCII letters and digits and **MUST** begin
with a lowercase letter. The name is case-sensitive and is used verbatim as the suffix of the
`match_<matcher-type>` subscribe query parameter (see (#subscription)) and as the `match_type`
value in authorization details (see (#topic-matcher-list)). The designated experts verify that
the defining specification states the matching semantics precisely, bounds the evaluation cost
of a crafted matcher (see (#url-pattern-denial-of-service) for the kind of exposure to
consider), and does not conflict with the reserved wildcard `*` (see (#matcher-types)).

Initial registrations:

| Matcher Type Name | Reference                             |
|-------------------|---------------------------------------|
| `exact`           | This specification, (#exact-matching) |
| `urlpattern`      | This specification, (#url-pattern)    |

## Mercure Actions Registry

IANA is requested to establish a "Mercure Actions" registry for the values of the `actions`
property of `mercure` authorization details (see (#authorization-details)). New registrations
follow the Specification Required policy [@!RFC8126].

An action name **MUST** consist of lowercase ASCII letters and digits. Hubs ignore actions
they do not recognize (see (#authorization-details)), so registering a new action does not
require deployed hubs to change.

Initial registrations:

| Action      | Reference                          |
|-------------|------------------------------------|
| `publish`   | This specification, (#publishers)  |
| `subscribe` | This specification, (#subscribers) |

# Security Considerations

The recommendations of the OAuth 2.0 Security Best Current Practice [@RFC9700] apply to
deployments of this protocol; this section highlights the considerations specific to Mercure.

The confidentiality of the secret key(s) used to sign access tokens is a primary concern. Such
keys **MUST** be stored securely and **MUST** be revoked immediately in the event of a breach.

A valid access token allows any client that holds it to subscribe to or publish on the hub. Its
confidentiality **MUST** therefore be ensured: access tokens **MUST** only be transmitted over
secure connections.

When the client is a web browser, the access token **SHOULD NOT** be exposed to JavaScript, to
provide resilience against cross-site scripting (XSS) attacks [@OWASP-XSS].
For this reason, `HttpOnly` cookies **SHOULD** be preferred as the authorization mechanism in
that case.

In the event of a breach, revoking access tokens before their expiration is often difficult.
Short-lived tokens are therefore strongly **RECOMMENDED**.

The hub's publishing endpoint can be targeted by cross-site request forgery (CSRF) attacks
[@OWASP-CSRF] when the cookie-based authorization mechanism is used. Implementations supporting
that mechanism **MUST** mitigate such attacks: the `SameSite` cookie attribute recommended in
(#cookie) is the first line of defense, and because some deployed user agents do not enforce
it, hubs **SHOULD** also verify that the source origin conveyed by the `Origin` or `Referer`
HTTP header matches the target origin, rejecting the request when neither header is available.
CSRF prevention techniques are described in depth in [@OWASP-CSRF-Prevention].

Access tokens **SHOULD NOT** be passed in page URLs (for example, via the `access_token` query
parameter). Browsers, web servers, and other software may not adequately secure URLs stored in
browser history, server logs, and other data structures, and an attacker able to read those
locations could steal the token [@RFC9700].

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
(#publication) prevent this injection. The `data` field may contain line breaks legitimately;
it is not constrained the same way, so the hub **MUST** serialize it as one `data:` field per
line (see (#publication)) rather than emitting the raw value. A hub that writes the value
without this line-splitting would let a `data` value containing `\nevent:` or `\nid:` inject a
forged field, so this serialization is a security requirement, not only a formatting one.

## Reserved Hub Namespace

The subtree of the hub URL's path (`/.well-known/mercure/` for the default hub URL) is
reserved for resources generated by the hub
itself (see (#publication)). A publisher with broad scope publishing under this prefix could
forge subscription events (see (#subscription-events)) and mislead other subscribers tracking
subscription lifecycle. The reserved-namespace test is applied to the topic's path component
after resolution against the hub's URL and after percent-encoding normalization, not as a
leading-substring match on the raw value; a
substring test would let an absolute topic addressing the hub's own host (for example,
`https://hub.example.com/.well-known/mercure/subscriptions/...`) bypass it, and skipping
normalization would let percent-encoded variants (for example, `/.well-known/%6Dercure/...`)
do the same.

## Authorization on Event Replay

When a subscriber reconnects with a `Last-Event-ID` header or `last_event_id` query parameter,
the same authorization rules apply to replayed events as to live events (see (#reconciliation)
and (#subscribers)). A subscriber whose authorized scope has shrunk between publication and
reconnection does not receive private events outside its scope at reconnection time. The
`Mercure-Last-Event-ID` response field is a cursor, however, and **MAY** contain the identifier
of an event the subscriber is not authorized to receive. Operators handling sensitive private updates
**SHOULD** use opaque, random event identifiers so that this identifier discloses nothing
beyond the event's existence.

## Subscriber Identifier Assignment

The `{subscriber}` identifier in subscription event topic URLs is either generated by the hub
or derived from information the hub has cryptographically validated, typically the issuer of
the subscriber's token together with its `sub` claim (see (#subscription-events)). Allowing
clients to supply, suggest, or
override this value through any unauthenticated channel would enable spoofing of subscription
events and hijacking of subscription state belonging to other subscribers. Deriving the
identifier from `sub` alone would let subscribers of distinct issuers collide with — and
thereby impersonate — one another, which is why the derivation incorporates the issuer.
Identifiers derived
from `sub` additionally disclose that claim's value to every subscriber authorized for the
corresponding subscription events; hub-generated identifiers avoid that disclosure.

## Private Update Authorization

A private update has exactly one topic, and the hub delivers it to a subscriber only when the
subscriber's access token grants the `subscribe` action on that topic (see (#subscribers) and
(#authorization-details)). Authorization is therefore a hub-enforced, per-resource check tied to
the token issued for the subscriber, not a property of the publisher's topic construction or of
the subscriber's routing matchers. Issuers scope each token's authorization details to the
resources a subscriber may read; the hub never widens that based on routing. Unauthenticated
subscribers, when the hub accepts them (see (#authorization)), present no token and therefore
never receive private updates.

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
pattern length, concurrent subscriptions per token, and concurrent connections per client and
in total (see (#subscription)) — bound this exposure.

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

# Privacy Considerations

The general privacy guidance of [@RFC6973] applies. The following considerations are specific
to this protocol:

*   Subscription events and the subscription API expose presence information: which subscriber
    identifiers hold which subscriptions, and when those subscriptions are created and
    terminated. Hubs restrict this information to authorized subscribers (see
    (#subscription-events)); token issuers **SHOULD NOT** grant `subscribe` on the
    subscriptions namespace more broadly than the tracking use case requires.
*   Payloads embedded in access tokens are broadcast to every subscriber authorized for the
    corresponding subscription events (see (#payloads) and (#payload-privacy)).
*   Topic identifiers appear in URLs (subscription query parameters and subscription event
    topics) and are therefore visible to the hub operator and to any infrastructure that logs
    URLs. Sensitive information **SHOULD NOT** be encoded in topic strings.
*   The hub operator observes connection metadata — client addresses, connection times, topic
    matchers, and update traffic patterns — even when update contents are encrypted. Update
    encryption (see (#encryption)) bounds what a curious or compromised hub learns to this
    metadata.
*   Event identifiers act as cursors that can reveal the existence and approximate timing of
    updates a subscriber is not authorized to read; see (#authorization-on-event-replay).

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

Note: entries reporting compatibility with revision 5 of this draft predate the OAuth 2.0
authorization model and the topic matcher types defined by this revision; they interoperate
only with hubs implementing those earlier revisions.

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

<reference anchor="urlpattern" target="https://urlpattern.spec.whatwg.org/review-drafts/2025-09/">
    <front>
        <title>URL Pattern Living Standard (Review Draft, September 2025)</title>
        <author>
            <organization>The Web Hypertext Application Technology Working Group (WHATWG)</organization>
        </author>
        <date year="2025" month="September"/>
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

<reference anchor="HTML" target="https://html.spec.whatwg.org/review-drafts/2026-01/">
    <front>
        <title>HTML Living Standard (Review Draft, January 2026)</title>
        <author>
            <organization>The Web Hypertext Application Technology Working Group (WHATWG)</organization>
        </author>
        <date year="2026" month="January"/>
    </front>
</reference>

<reference anchor="URL" target="https://url.spec.whatwg.org/review-drafts/2026-02/">
    <front>
        <title>URL Living Standard (Review Draft, February 2026)</title>
        <author>
            <organization>The Web Hypertext Application Technology Working Group (WHATWG)</organization>
        </author>
        <date year="2026" month="February"/>
    </front>
</reference>

<reference anchor="xhr" target="https://xhr.spec.whatwg.org/review-drafts/2025-08/">
    <front>
        <title>XMLHttpRequest Living Standard (Review Draft, August 2025)</title>
        <author>
            <organization>The Web Hypertext Application Technology Working Group (WHATWG)</organization>
        </author>
        <date year="2025" month="August"/>
    </front>
</reference>

<reference anchor="FETCH" target="https://fetch.spec.whatwg.org/review-drafts/2026-06/">
    <front>
        <title>Fetch Living Standard (Review Draft, June 2026)</title>
        <author>
            <organization>The Web Hypertext Application Technology Working Group (WHATWG)</organization>
        </author>
        <date year="2026" month="June"/>
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

<reference anchor="UNICODE" target="https://www.unicode.org/versions/Unicode17.0.0/">
    <front>
        <title>The Unicode Standard, Version 17.0.0</title>
        <author>
            <organization>The Unicode Consortium</organization>
        </author>
        <date year="2025" month="September"/>
    </front>
</reference>

<reference anchor="streams" target="https://streams.spec.whatwg.org/">
    <front>
        <title>Streams Living Standard</title>
        <author>
            <organization>The Web Hypertext Application Technology Working Group (WHATWG)</organization>
        </author>
        <date year="2026"/>
    </front>
</reference>

<reference anchor="OWASP-XSS" target="https://owasp.org/www-community/attacks/xss/">
    <front>
        <title>Cross Site Scripting (XSS)</title>
        <author>
            <organization>OWASP Foundation</organization>
        </author>
        <date year="2024"/>
    </front>
</reference>

<reference anchor="OWASP-CSRF" target="https://owasp.org/www-community/attacks/csrf">
    <front>
        <title>Cross Site Request Forgery (CSRF)</title>
        <author>
            <organization>OWASP Foundation</organization>
        </author>
        <date year="2024"/>
    </front>
</reference>

<reference anchor="OWASP-CSRF-Prevention" target="https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html">
    <front>
        <title>Cross-Site Request Forgery Prevention Cheat Sheet</title>
        <author>
            <organization>OWASP Foundation</organization>
        </author>
        <date year="2024"/>
    </front>
</reference>

<reference anchor="EventSourcing" target="https://martinfowler.com/eaaDev/EventSourcing.html">
    <front>
        <title>Event Sourcing</title>
        <author initials="M." surname="Fowler" fullname="Martin Fowler"/>
        <date year="2005" month="December"/>
    </front>
</reference>

{backmatter}

# Changes from Pre-Standardization Deployments

This appendix is non-normative. It summarizes the wire-level differences between this
specification and the pre-standardization revisions of Mercure still deployed in the wild, to
help implementers migrate. It complements the version-detection note in (#discovery): a hub that
publishes no protected resource metadata (see (#protected-resource-metadata)) likely implements
one of these earlier revisions.

| Aspect | Pre-standardization | This specification |
|--------|---------------------|--------------------|
| Authorization grant | `mercure` JWT claim with `publish` and `subscribe` arrays of topic selectors | RFC 9396 `authorization_details` entry with `type` `mercure`, an `actions` array (`publish`, `subscribe`), and a `topics` array (see (#authorization-details)) |
| Token type | any signed JWT | JWT access token with `typ` `at+jwt` (see (#token-validation)) |
| Topic matching | raw topic selectors passed as `topic` query parameters | `exact` and `urlpattern` matcher types selected with the `match`/`match_<matcher-type>` query parameters or the `match_type` member (see (#matcher-types)) |
| Reconnection query parameter | `Last-Event-ID` | `last_event_id` (see (#reconciliation)) |
| Reconnection response field | `Last-Event-ID` | `Mercure-Last-Event-ID` (see (#reconciliation)) |
| Authorization cookie | `mercureAuthorization` | `__Secure-mercure_access_token` (see (#cookie)) |
