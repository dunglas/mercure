%%%
title = "The Mercure Protocol"
area = "Applications and Real-Time"
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

# Terminology

The keywords **MUST**, **MUST NOT**, **REQUIRED**, **SHALL**, **SHALL NOT**, **SHOULD**, **SHOULD
NOT**, **RECOMMENDED**, **MAY**, and **OPTIONAL**, when they appear in this document, are to be
interpreted as described in [@!RFC2119].

*   Topic: The unit to which one can subscribe for changes. The topic is identified by a string
    that **MAY** be an IRI [@!RFC3987]. Topic strings **MUST** be valid UTF-8 [@!RFC3629] and
    **MUST NOT** contain C0 control characters (U+0000–U+001F) or U+007F (DEL).
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
the publisher; see (#discovery)) following the Server-Sent Events specification
[@!W3C.REC-eventsource-20150203]. The `GET` HTTP method **MUST** be used. The connection
**SHOULD** use HTTP version 2 or higher to leverage multiplexing and other performance-related
features.

The subscriber specifies the list of topics to receive updates from using one or more query
parameters named `match`.

The subscriber can also use other matcher types via query parameters whose name is `match`
concatenated with the matcher type name (e.g., `matchRegexp`, `matchURLPattern`).

The `matchExact` query parameter **MUST** be treated as equivalent to `match`.

The names of topic matcher query parameters are case-sensitive. They **MUST** be either a
query parameter name defined in the "Mercure Matcher Types" registry (see
(#iana-matcher-types)), or the concatenation of `match` and a hub-supported non-registered
matcher type name (see (#other-matcher-types)). Requests using a name that is neither
registered nor implemented by the hub **MUST** be rejected with a 400 "Bad Request" HTTP
status code.

If the type of one or more matchers is not supported by the hub, it **MUST** respond with a
501 "Not Implemented" HTTP status code.

The value of each topic matcher query parameter **MUST** be valid UTF-8 [@!RFC3629] and
**MUST NOT** contain C0 control characters or U+007F. Requests violating this constraint
**MUST** be rejected with a 400 "Bad Request" HTTP status code.

The subscriber receives updates for all topics matching at least one topic matcher according to
the matcher type rules.

To mitigate resource exhaustion, hubs **SHOULD** apply implementation-defined maximums to the
number of topic matcher query parameters in a single request and to the length of each
matcher's pattern. Requests exceeding any such limit **MUST** be rejected with a 400 "Bad
Request" HTTP status code. A subscription is created for every registered topic matcher query
parameter present in the request (see (#iana-matcher-types)). Hubs **MAY** deduplicate
subscriptions that have identical matcher type and pattern. See (#subscription-events).

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
exact, case-sensitive, byte-for-byte comparison between the topic and the matcher. The hub
**MUST NOT** resolve relative values against the hub's URL or any other base, and **MUST NOT**
perform Unicode or IRI normalization.

Note: Because comparison is performed on raw bytes, publishers and subscribers **SHOULD**
normalize topic strings to a canonical form before publication or subscription. Recommended
canonicalizations are Unicode NFC [@!UNICODE] and, for IRIs, IDNA-canonical hosts [@!RFC5891]
and percent-encoding normalization [@!RFC3986]. Otherwise, visually identical topics will be
treated as distinct, and homograph attacks (see (#security-considerations)) become possible.

The matcher type name is `Exact`.
The corresponding query parameters are `match` and `matchExact`.

## URL Pattern

The hub **SHOULD** support using URL patterns [@!urlpattern] as matchers.
URL patterns **SHOULD** be preferred to regular expressions when matching URLs.

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
by clients submitting pathological patterns, hubs implementing URL Pattern matchers **MUST**
either use a regular expression engine that guarantees linear-time matching (such as RE2 [@re2])
or enforce an implementation-defined evaluation cost or time limit. When such a limit is
reached, the pattern **MUST** be treated as not matching and the evaluation **MUST** be aborted.

URL patterns whose `protocol` component is a wildcard or capture group can match `data:`,
`javascript:`, `file:`, and other potentially dangerous URI schemes. Topic strings are opaque
identifiers within this protocol; subscribers **MUST NOT** dereference them as URLs without
validating the scheme against an allowlist appropriate for the subscriber's environment.

The matcher type name is `URLPattern`.
The corresponding query parameter is `matchURLPattern`.

## Regular Expression

The hub **SHOULD** support using I-Regexp regular expressions [@!RFC9485] as matchers.
The hub **MUST NOT** resolve relative values against the hub's URL or any other base.

I-Regexp defines a dialect but not an evaluation engine. To mitigate denial-of-service attacks
by clients submitting pathological expressions, hubs **MUST** either use a regular expression
engine that guarantees linear-time matching (such as RE2 [@re2]) or enforce an
implementation-defined evaluation cost or time limit. When such a limit is reached, the
expression **MUST** be treated as not matching and the evaluation **MUST** be aborted.

The matcher type name is `Regexp`.
The corresponding query parameter is `matchRegexp`.

## Common Expression Language (CEL)

The hub **MAY** support using CEL expressions [@cel] as matchers.

A variable named `topics` containing an array of strings **MUST** be passed to the expression.
This variable **MUST** contain the canonical topic followed by the alternate topics of the
update to match.

The hub **MAY** also pass implementation-specific variables and expose implementation-specific
functions. Implementation-specific functions **MUST** be deterministic and side-effect-free;
they **MUST NOT** perform network requests, access the filesystem, execute external processes,
read clocks or random sources, or expose any other operation with externally-observable side
effects. Exposing such operations would allow a malicious subscriber's expression to perform
server-side request forgery or read sensitive material from the hub's environment.

Hubs **SHOULD** apply an implementation-defined maximum size to the `topics` array.

The expression **MUST** return a boolean value: `true` if the topic matches, `false` otherwise.

If parsing or checking of a CEL expression fails, or if the expression does not return a boolean
value, the hub **MUST** return a 400 "Bad Request" HTTP status code.

To mitigate denial-of-service attacks by clients submitting pathological expressions,
hubs implementing CEL **MUST** enforce an implementation-defined evaluation cost limit.
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

The hub **MAY** implement additional matcher types. Matcher type names intended for
interoperable deployment **MUST** be registered in the "Mercure Matcher Types" registry; see
(#iana-matcher-types).

Names for non-registered, hub-specific matcher types **MUST** be prefixed by a token under
the implementer's exclusive control, separated from the type-specific suffix by a `.`
(U+002E) character. The prefix **SHOULD** be either a DNS-controlled domain name in
reverse-dotted order (for example, `com.example.Foo`) or another globally-unique identifier
such as a registered trademark (for example, `ACME.Foo`). The intent is to minimize the
collision risk that motivated [@!RFC6648] to deprecate the `X-` convention for HTTP headers.

Names that match the production for a registered matcher type (case-sensitive PascalCase
ASCII without `.`) **MUST NOT** be used for non-registered types. The corresponding query
parameter name is the concatenation of `match` and the non-registered name as given
(for example, `matchcom.example.Foo`).

Implementations of non-registered matcher types **MUST** apply the same denial-of-service and
sandbox controls as the corresponding built-in matcher with the closest evaluation model. In
particular, expression-based or pattern-based custom matchers **MUST** use either an engine
that guarantees linear-time matching or an implementation-defined evaluation cost or time
limit, and **MUST NOT** expose network requests, filesystem access, process execution, or
other externally-observable side effects to the expression.

A custom matcher **MUST NOT** produce an authorization decision that would not also be reached
by some registered matcher type for the same `match` value and topic. In other words, custom
matchers extend the set of usable patterns; they **MUST NOT** subvert authorization checks
defined in (#authorization).

JWSs containing non-registered matcher type names are not portable across hubs. Issuers and
clients using such names accept that subscriptions and publications dependent on those names
will be rejected with a 501 "Not Implemented" HTTP status code by hubs that do not implement
the same name with compatible semantics.

# Publication

The publisher sends updates by issuing `POST` HTTPS requests to the hub URL. When it receives an
update, the hub dispatches it to subscribers using the established server-sent events connections.

The hub **MAY** also dispatch the update using other protocols such as WebSub
[@W3C.REC-websub-20180123] or ActivityPub [@W3C.REC-activitypub-20180123].

An application **MAY** deliver events directly to subscribers without an external hub. In that
case, the publish endpoint described in this section is not required.

The request **MUST** be encoded using the `application/x-www-form-urlencoded` format
[@W3C.REC-html52-20171214] and **MUST** contain at least one `topic` field. Field names and
values **MUST** be UTF-8 [@!RFC3629]. It **MAY** also contain the following name-value tuples:

*   `topic`: The identifiers of the updated topic. It is **RECOMMENDED** to use an IRI as
    identifier. If this name is present several times, the first occurrence is the canonical IRI
    of the topic and the remaining ones are alternate IRIs. The hub **MUST** dispatch the update
    to subscribers that are subscribed to either the canonical IRI or any of its alternate IRIs.
    Topic values **MUST** conform to the constraints defined in (#terminology). Topic values
    (canonical or alternate) **MUST NOT** start with the prefix `/.well-known/mercure/`; this
    namespace is reserved for resources generated by the hub itself, including subscription
    events (see (#subscription-events)). Hubs **MUST** reject publish requests violating this
    rule with a 403 HTTP status code.
*   `data` (optional): the content of the new version of this topic. The value **MUST** be
    valid UTF-8 [@!RFC3629].
*   `private` (optional): if this field is present, the update **MUST NOT** be dispatched to
    subscribers not authorized to receive it. See (#authorization). The presence of the field
    name marks the update as private regardless of its value; hubs **MUST NOT** interpret the
    field's value to determine privacy. It is **RECOMMENDED** to set the value to `on` for
    interoperability, but it **MAY** contain any value, including an empty string.
*   `id` (optional): the topic's revision identifier; used as the SSE `id` property.
    The provided ID **MUST NOT** start with the `#` character and **MUST NOT** contain U+000A
    (LF), U+000D (CR), or U+0000 (NUL). The provided ID **MAY** be a valid IRI. If omitted, the
    hub **MUST** generate a valid IRI [@!RFC3987]. A UUID [@RFC4122] or a
    [DID](https://www.w3.org/TR/did-core/) **MAY** be used. Alternatively, the hub **MAY**
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

On success, the hub **MUST** return a successful HTTP status code, and the response body **MUST**
be the `id` generated by the hub for the update. The publisher **MUST** be authorized to publish
updates; see (#authorization).

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

To prove that they are authorized, both publishers and subscribers **MUST** present a valid JWS
[@!RFC7515] in compact serialization to the hub. This JWS **SHOULD** be short-lived, especially
when the subscriber is a web browser. Different keys **SHOULD** be used to sign subscribers' and
publishers' tokens so that compromise of one role does not entail compromise of the other.

Note: Hubs **MAY** be deployed without requiring authorization (for example, when serving only
publicly-readable updates over a trusted network). Such deployments fall outside the scope of
the rest of this section. They **MUST NOT** be reachable from networks containing untrusted
clients, since any client able to reach the hub will be able to publish and subscribe at will.
The remainder of this section assumes JWS-based authorization is in use.

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

If no JWS is presented, or if the JWS fails validation as defined in (#jws-validation), the hub
**MUST** return a 401 "Unauthorized" HTTP status code. If a valid JWS is presented but does not
authorize the requested operation, the hub **MUST** return a 403 "Forbidden" HTTP status code.
Hubs **SHOULD NOT** disclose, in error responses, whether the failure was due to JWS validation
or to insufficient scope.

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

The cookie **MUST** have the `Secure` and `HttpOnly` attributes set. The cookie **SHOULD** also
have `SameSite=Strict`; `SameSite=Lax` **MAY** be used if cross-site discovery flows require it.
The cookie's `Path` attribute **SHOULD** also be set to the hub's URL. See
(#security-considerations).

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

## JWS Validation {#jws-validation}

Hubs **MUST** validate JWSs in accordance with the JSON Web Token Best Current Practices
[@!RFC8725]. In particular:

*   Hubs **MUST NOT** accept JWSs with `alg=none` and **MUST** verify that the `alg` header
    parameter is compatible with the key type used for signature verification (preventing
    algorithm-confusion attacks).
*   Hubs **MUST** enforce the `exp` claim [@!RFC7519] if present, including on the first
    request received bearing a JWS, and **MUST** enforce the `nbf` claim if present.
*   If the hub publishes an identifier (e.g., its canonical URL) for use in the `aud` claim,
    and the JWS contains an `aud` claim, the hub **MUST** verify that this identifier appears
    in `aud`.
*   Hubs **SHOULD** support at minimum the algorithms `EdDSA`, `ES256`, and `RS256`, and
    **MUST NOT** accept any algorithm whose security has been compromised at the time of
    deployment.

Failure of any of these checks **MUST** be treated as JWS validation failure and **MUST** be
reported as defined in the introduction to this section.

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

Topic matchers present in the `mercure.subscribe` or `mercure.publish` claim **MUST** be JSON
objects.

Each object **MUST** have a `match` property containing the topic matcher itself, and **MAY**
have an OPTIONAL `matchType` property containing the topic matcher type. The value of
`matchType` is case-sensitive and **MUST** match exactly one of the matcher type names defined
in (#matcher-types) (e.g., `Exact`, `URLPattern`, `Regexp`, `CEL`, `URITemplate`) or a name
implemented by the hub. If no `matchType` key is present, the hub **MUST** assume the `Exact`
matcher type.

Any entry that is not a JSON object **MUST** be rejected with a 400 "Bad Request" HTTP status
code. Earlier drafts of this protocol allowed matchers to be expressed as bare strings;
silently reinterpreting them under the rules defined here could change the semantics of tokens
minted for those earlier versions.

If the type of one or more matchers in `mercure.subscribe` is not supported by the hub, the hub
**MUST** reject the subscription request with a 501 "Not Implemented" HTTP status code and
**MUST NOT** establish the subscription.

If the type of one or more matchers in `mercure.publish` is not supported by the hub, the hub
**MUST** reject the publication request with a 501 "Not Implemented" HTTP status code and
**MUST NOT** dispatch the update.

If any entry in `mercure.subscribe` or `mercure.publish` fails to parse or validate as a topic
matcher (including failures specific to its `matchType`), the hub **MUST** reject the request
with a 400 "Bad Request" HTTP status code and **MUST NOT** establish a subscription or dispatch
an update on the basis of the remaining entries. Partial acceptance of a matcher list is
forbidden because it would silently alter the effective authorization scope of the JWS.

Hubs **SHOULD** apply implementation-defined maximums to the number of entries in
`mercure.subscribe` and `mercure.publish` and to the length of individual patterns. Tokens
exceeding any such limit **MUST** be rejected with a 400 HTTP status code.

Hubs **MAY** also limit the number of concurrent subscriptions established under a single JWS
and **MAY** reject further subscription attempts with a 429 "Too Many Requests" HTTP status
code once the limit is reached.

## Payloads

User-defined data can be attached to subscriptions and made available through the subscription
API and in subscription events. See (#subscription-events).

Each entry in the `mercure.subscribe` claim **MAY** contain a JSON object under the `payload`
key, providing payload data scoped to that topic matcher.

The `mercure` claim **MAY** also contain a top-level `payload` key holding a JSON object. This
top-level payload is used as a default when no per-matcher payload applies.

The `payload` value associated with the first topic matcher in the `mercure.subscribe` claim
that matches the subscription's own matcher (as determined by the `match` and `matchType` query
parameters) **MUST** be included under the `payload` key in the JSON object describing a
subscription, both in the subscription API and in subscription events.

A claim matcher is considered to match a subscription matcher when any of the following holds:

1.  The claim matcher's type is the same as the subscription matcher's type and both patterns
    are byte-identical.
2.  The claim matcher, evaluated against the subscription matcher's `match` value (treated as
    an opaque string regardless of the subscription matcher's type), returns true. For
    instance, a claim with `matchType=URLPattern` and `match=https://example.com/:id` matches a
    subscription with `matchType=Exact` and `match=https://example.com/42`, because the URL
    pattern accepts that URL string.

If no claim matches the subscription, the hub **MUST** fall back to the top-level
`mercure.payload` value, if any.

Note: Payload selection is order-dependent; the first matching entry in `mercure.subscribe`
wins. Issuers placing broad catchall matchers before more specific entries will mask the
payloads of the specific entries. Specific matchers **SHOULD** appear before broader ones.

Privacy: Payloads are forwarded to other authorized subscribers via subscription events (see
(#subscription-events)). Issuers **MUST NOT** place data in payloads that should not be visible
to other subscribers authorized for the corresponding subscription events. In particular,
storing data identifying the subscriber (such as a user identifier or IP address) effectively
broadcasts that data to all other subscribers within the same subscription-events scope.

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

For instance, a payload can carry coarse-grained metadata such as a tenant identifier or a
display label for the subscription. Issuers **MUST** consider the privacy note above before
including any identifier of the subscriber, since payloads are visible to other authorized
subscribers via subscription events.

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
**SHOULD** send all events published after the one bearing this identifier to the subscriber,
subject to authorization.

The authorization rules defined in (#subscribers) apply to replayed events identically to live
events: the hub **MUST** re-evaluate each candidate replayed event against the current JWS
scope before dispatching it. Events whose `private` flag is set and that do not satisfy the
current `mercure.subscribe` claim **MUST NOT** be dispatched, regardless of any authorization
that may have applied at publication time.

The reserved value `earliest` requests that the hub send all updates it has for the subscribed
topics. The hub **MAY** ignore this request according to its own policy.

The hub **MAY** discard some events for operational reasons. When the request contains a
`Last-Event-ID` HTTP header or a `lastEventID` query parameter, the hub **MUST** set a
`Last-Event-ID` header on the HTTP response.

The value of this response header **MUST** be one of:

*   the identifier of the most recent event preceding the first event sent to the subscriber,
    provided that the subscriber is authorized to receive that event and that the hub located
    it within an implementation-defined backward-scan limit; or
*   the reserved value `earliest`, in every other case (for example, when the hub history is
    empty, when the requested event does not exist or has been discarded, when the scan
    limit was reached without finding a visible event, or when all candidate predecessors
    are private updates the subscriber is not authorized to receive).

The hub **MUST NOT** disclose, via this header, the identifier of any event the subscriber
is not authorized to receive. To bound the cost of authorization filtering on candidate
predecessor events, hubs **SHOULD** apply an implementation-defined backward-scan limit;
once the limit is reached the response value **MUST** be `earliest`. Hubs whose storage
backend supports both cheap predecessor seek (e.g., Redis Streams `XREVRANGE`) and cheap
per-event authorization filtering **MAY** omit the limit. Subscribers using the hub as an
event store can use the returned predecessor identifier as a recovery anchor; subscribers
that only need to detect data loss can compare the response value against the requested
value (a different value indicates that loss may have occurred).

Note: The privacy impact of predecessor-identifier disclosure depends on the hub's identifier
generation strategy. Opaque random identifiers (e.g., UUIDv4 [@!RFC4122]) leak only the
existence of an event the subscriber cannot read; time-ordered or sequential identifiers
(e.g., UUIDv7, Redis Stream IDs, snowflake IDs) additionally leak the timing and ordering of
such events. Operators handling highly sensitive private updates **SHOULD** prefer opaque
random identifiers, even though those are less convenient as event-store cursors.

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
    the `sub` claim of the subscriber's JWS (after validation per (#jws-validation)), but
    other authenticated values **MAY** be used. The hub **MUST** ensure that the identifier
    is unique among active subscriptions.

Note: Because strings containing reserved characters (e.g., URIs, URL Patterns, and URI
Templates) can be used for the `{match}` and `{subscriber}` variables, per [@!RFC6570] the
values of all variables **MUST** be percent-encoded during expansion.

If a subscriber has several subscriptions, the hub **SHOULD** assign the same `{subscriber}`
value to all of them when it can correlate them (for example, when the same `sub` claim is
presented across requests).

`{subscriber}` **SHOULD** be an IRI [@!RFC3987]. A UUID [@RFC4122] or a
[DID](https://www.w3.org/TR/did-core/) **MAY** also be used.

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

Subscribers parsing JSON-LD documents produced by the hub **SHOULD NOT** automatically
dereference the `@context` URL. The context below is fixed and **MAY** be embedded in client
implementations or cached locally; this avoids both unnecessary network requests and the risk
that an attacker able to control responses from the context URL alters the semantics of
subsequently parsed documents.

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

Note: A compromised publisher can advertise a malicious hub URL and capture the JWSs of
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
*   `key-set`: the URL of the key set used to decrypt updates, encoded in the JWK Set (JSON Web
    Key Set) format [@!RFC7517]. See (#encryption). Because this key set contains secret key
    material, the publisher **MUST** restrict access to this URL to authorized subscribers
    only. The authorization mechanism described in (#authorization) **MAY** be reused for this
    purpose, but the publisher is responsible for implementing the access control on its own
    endpoint; this is not the hub's responsibility. Misconfigured access control on the
    `key-set` URL defeats the encryption protections described in (#encryption).

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

## JSON Web Token (JWT) Registry

A new "JSON Web Token Claim" as described in (#authorization) is to be registered in the
"JSON Web Token Claims" registry with the following entry:

*   Claim Name: mercure
*   Description: Mercure data.
*   Reference: This specification, (#authorization)

## Mercure Matcher Types {#iana-matcher-types}

IANA is requested to create a new registry titled "Mercure Matcher Types", to be maintained
under the "Mercure Protocol Parameters" group.

The registration policy for this registry is "Specification Required" [@!RFC8126]. Each
registration defines a matcher type by a case-sensitive PascalCase name and the case-sensitive
query parameter name used to request matchers of that type in a subscription request. The
query parameter name **MUST** be the concatenation of the string `match` and the matcher type
name. Implementations **MUST** treat registered names exactly as listed; case-folded variants
are not equivalent.

The change controller for all initial entries is the IETF. The initial contents of the
registry are:

| Matcher Type   | Query Parameter               | Reference                                              |
|----------------|-------------------------------|--------------------------------------------------------|
| `Exact`        | `matchExact` (alias: `match`) | This specification, (#exact-matching)                  |
| `URLPattern`   | `matchURLPattern`             | This specification, (#url-pattern)                     |
| `Regexp`       | `matchRegexp`                 | This specification, (#regular-expression)              |
| `CEL`          | `matchCEL`                    | This specification, (#common-expression-language-cel)  |
| `URITemplate`  | `matchURITemplate`            | This specification, (#uri-template)                    |

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

## JWS Validation

Hubs **MUST** validate JWS structure, signature, and standard claims as defined in
(#jws-validation) before evaluating the `mercure` claim. Failure to reject `alg: none`, to
bind `alg` to key type, or to enforce `exp` and `nbf` enables token forgery, replay across
contexts, and algorithm-confusion attacks on JWS.

## Server-Sent Events Field Injection

Topic strings and the `id`, `type`, and `retry` publish fields end up on the wire as part of
the Server-Sent Events framing. Values containing CR (U+000D), LF (U+000A), or NUL (U+0000)
can inject arbitrary SSE fields into the stream as seen by subscribers, including forged event
identifiers and event types. Hubs **MUST** reject such values; see (#terminology) and
(#publication).

## Reserved Hub Namespace

The URL path prefix `/.well-known/mercure/` is reserved for resources generated by the hub
itself. Publishers **MUST NOT** publish updates whose canonical or alternate topic falls under
this prefix; otherwise a publisher with broad scope could forge subscription events (see
(#subscription-events)) and mislead other subscribers tracking subscription lifecycle.

## Authorization on Event Replay

When a subscriber reconnects with a `Last-Event-ID` header or `lastEventID` query parameter,
the hub **MUST** apply the same authorization rules to replayed events as to live events. A
subscriber whose authorized scope has shrunk between publication and reconnection **MUST NOT**
receive private events it would not be authorized to receive at the time of reconnection. The
identifier of a private event the subscriber is not authorized to receive **MUST NOT** appear
in the `Last-Event-ID` response header; see (#reconciliation).

## Subscriber Identifier Assignment

The `{subscriber}` identifier in subscription event topic URLs **MUST** be derived from
information the hub has cryptographically validated, typically the `sub` claim of the
subscriber's JWS. Allowing clients to supply, suggest, or override this value through any
unauthenticated channel enables spoofing of subscription events and hijacking of subscription
state belonging to other subscribers. See (#subscription-events).

## Regular-Expression and URL-Pattern Denial of Service

I-Regexp [@!RFC9485] is a dialect, not an evaluation engine. URL Pattern internally compiles
to a regular expression. Naive implementations on engines such as PCRE are vulnerable to
catastrophic backtracking. Hubs **MUST** use an engine that guarantees linear-time matching
(such as RE2 [@re2]) or enforce per-evaluation cost or time limits; see (#regular-expression)
and (#url-pattern).

## CEL Sandbox

Implementation-specific functions exposed to CEL expressions **MUST** be deterministic and
side-effect-free, and **MUST NOT** perform network requests, filesystem access, process
execution, or any other externally-observable operation. Exposing such operations enables a
malicious subscriber's expression to perform server-side request forgery, read sensitive
material from the hub's environment, or amplify denial-of-service attacks. See
(#common-expression-language-cel).

## Payload Privacy

Per-matcher and top-level payloads carried in `mercure.subscribe` are included in subscription
events and forwarded to other authorized subscribers. Issuers **MUST** treat payloads as
broadcast data within the set of subscribers authorized for the corresponding subscription
events, not as private metadata about an individual subscriber. See (#payloads).

## Topic Normalization

Topic strings are compared as byte sequences. Without Unicode normalization (NFC) and IDNA
host canonicalization, visually identical topics may be treated as distinct, leading to
undelivered updates or to spoofable topic names through homograph attacks (e.g.,
`example.com` versus a host containing Cyrillic look-alike characters). Publishers and
subscribers **SHOULD** normalize topic strings to a canonical form before publication or
subscription; see (#exact-matching).

## Resource Limits

Hubs **SHOULD** apply implementation-defined limits to: the size of publish request bodies,
the length of individual fields, the number of topic matcher query parameters per request,
the number of entries in `mercure.subscribe` and `mercure.publish`, the length of individual
matcher patterns, and the number of concurrent subscriptions per token. Absent such limits,
malicious clients can exhaust hub resources.

## Hub Trust

Subscribers obtain hub URLs from publishers via the discovery mechanism (see (#discovery)) and
transmit credentials to the hub. A compromised publisher can therefore redirect subscribers to
a hub of its choosing and capture those credentials. Subscribers **SHOULD** constrain the set
of hub origins they will connect to, and **MAY** verify hub identity out of band.

## JWE Algorithms and Replay

JWE-protected updates are subject to algorithm-selection pitfalls and to replay. Implementers
**MUST** restrict JWE algorithms to currently strong choices and **SHOULD** include freshness
indicators in encrypted payloads when replay is in scope; see (#encryption).

## Custom Matcher Types

Hub-specific matcher types extend the protocol surface and therefore the attack surface.
Misuse can cause authorization bypass through type confusion (a JWS minted for one hub's
custom type evaluated under a different semantics on another hub), denial of service through
unguarded pattern evaluation, and server-side request forgery or sensitive-material disclosure
through expression-based matchers exposing dangerous operations.

The rules in (#other-matcher-types) prevent these outcomes by mandating globally unique
naming (reverse-DNS or HTTPS URI form, syntactically distinct from registered names), parity
with built-in DoS and sandbox controls, and an authorization invariant requiring custom
matchers to remain consistent with registered matchers for the same `match` value. Hub
operators **MUST** review each custom matcher implementation against these requirements before
enabling it, and **SHOULD** document its semantics so that JWS issuers can scope tokens
correctly.

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
