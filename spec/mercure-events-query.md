%%%
title = "The Mercure Events Query Extension"
abbrev = "Mercure Events Query"
ipr = "trust200902"
area = "Web and Internet Transport"
submissiontype = "IETF"

[seriesInfo]
name = "Internet-Draft"
value = "draft-dunglas-mercure-events-query-00"
stream = "IETF"
status = "standard"

[[author]]
initials="K."
surname="Dunglas"
fullname="Kévin Dunglas"
role="editor"
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

This document defines an extension to the Mercure protocol that delivers updates as HTTP
Events Query notifications instead of Server-Sent Events. A subscriber creating its
subscription with the `QUERY` method can negotiate a `multipart/mixed` response in which every
update is a body part, bound the response duration with the `Events` header field, and resume
from the last received event. The extension also adds a `multipart/form-data` publication
encoding so that updates can carry arbitrary binary payloads and declare their media type.

This document updates the subscription and publication requirements of the Mercure protocol
for hubs that implement it.

{mainmatter}

# Introduction

The Mercure protocol [@!I-D.dunglas-mercure] delivers updates over Server-Sent Events [@!HTML]:
a text format, natively consumed by web browsers, that every hub provides and that remains the
interoperability baseline.

HTTP Events Query [@!I-D.gupta-httpapi-events-query] specifies a generic mechanism for
receiving notifications for events on a resource using the `QUERY` HTTP method [@!RFC10008].
A Mercure subscription request already realizes its subscription data model: the
form-encoded `QUERY` request body carries the subscription parameters (the topic matchers and
the `last_event_id` reconciliation cursor). This extension specifies the response side: how a
hub serves a Mercure subscription as an incremental `multipart/mixed` [@!RFC2046] response,
and how the Mercure reconciliation and authorization models apply to it unchanged.

Two capabilities motivate the extension beyond protocol alignment. First, MIME encapsulation
carries per-event metadata (the event ID and the payload media type) as part header fields and
the payload as raw bytes, where Server-Sent Events normalize line endings and define no
per-event metadata slot. Second, it can therefore deliver binary payloads verbatim; this
document pairs it with a `multipart/form-data` [@!RFC7578] publication encoding so that binary
payloads can also enter the hub.

Support for this extension is **OPTIONAL**. A hub that does not implement it remains fully
conformant to [@!I-D.dunglas-mercure]; a subscriber that does not use it observes no behavior
change, since the extension only activates through proactive content negotiation [@!RFC9110].

# Terminology

The key words **MUST**, **MUST NOT**, **REQUIRED**, **SHALL**, **SHALL NOT**, **SHOULD**,
**SHOULD NOT**, **RECOMMENDED**, **NOT RECOMMENDED**, **MAY**, and **OPTIONAL** in this document
are to be interpreted as described in BCP 14 [@!RFC2119] [@!RFC8174] when, and only when, they
appear in all capitals, as shown here.

This document uses the terms defined by [@!I-D.dunglas-mercure]: topic, update, subscriber,
publisher, and hub. A "binary update" is an update published with the `multipart/form-data`
encoding defined in (#binary-publication), whatever the actual content of its `data` value.

# Subscription Response Negotiation

The Mercure protocol requires hubs to send updates as `text/event-stream`-compliant events.
This document updates that requirement as follows: for subscriptions created with a safe
method carrying the parameters in the request body (notably `QUERY` [@!RFC10008]), a hub
implementing this extension **MAY** offer the `multipart/mixed` response media type, selected
through proactive content negotiation [@!RFC9110]. In the absence of an `Accept` header field
preferring `multipart/mixed`, the hub **MUST** send `text/event-stream`-compliant events as
before, so enabling the extension cannot change the behavior observed by existing subscribers.
Subscriptions created with the `GET` method remain `text/event-stream` only.

All requirements that [@!I-D.dunglas-mercure] expresses on subscription responses —
authorization, private update dispatching, reconciliation, the `Mercure-Last-Event-ID`
response header, CORS, and resource-exhaustion limits — apply identically to `multipart/mixed`
responses. Properties specific to the `text/event-stream` format (`type` and `retry`) have no
representation in this encoding.

The hub **SHOULD** send the `Incremental: ?1` response header field
[@!I-D.ietf-httpbis-incremental] to signal that the response is generated and consumed
incrementally, and **SHOULD** advertise the media type it accepts for `QUERY` request bodies
with an `Accept-Query: application/x-www-form-urlencoded` response header field [@!RFC10008].
Both header fields are also meaningful on `text/event-stream` responses and **MAY** be sent on
every subscription response.

# Notification Encoding

Each update is serialized as one body part of the `multipart/mixed` response:

*   The part body is the update's `data` value, conveyed verbatim, byte for byte. No
    transfer encoding is applied.
*   The `Content-Event-Id` part header field carries the update's `id` verbatim. This is a MIME
    extension field: [@!RFC2045] reserves all fields beginning with `Content-` for extension,
    and [@!RFC2046] gives body-part meaning to `Content-` fields only, while other fields may
    be ignored or discarded by gateways. The registered `Content-ID` field cannot carry a
    Mercure event ID (its `msg-id` syntax [@RFC5322] requires an `@` and forbids `:`, so IRIs
    such as `urn:uuid:...` are not valid values), and `Content-Location` cannot either (event
    IDs are not required to be URIs). The value is subject to the character constraints that
    [@!I-D.dunglas-mercure] places on event IDs, which exclude all control characters, so it
    is always a valid field value.
*   The `Content-Type` part header field carries the media type of the `data` value when the
    publisher declared one (see (#binary-publication)). When the publisher declared none, the
    part **MUST NOT** carry a `Content-Type` field: the hub treats `data` as opaque and does
    not guess.
*   A `Content-Length` part header field **MAY** be included so consumers can preallocate
    buffers; it is advisory, as MIME part framing is delimiter-based.

The part boundary **MUST** be generated so that it cannot occur in any payload, as required by
[@!RFC2046]; a sufficiently long random boundary satisfies this. The hub **MUST NOT** emit
anything between parts other than the delimiters themselves: `multipart/mixed` defines no
ignorable mid-stream filler, so there is no keep-alive mechanism inside the response (see
(#bounded-responses)).

Example:

~~~ http
QUERY /.well-known/mercure HTTP/1.1
Host: example.com
Accept: multipart/mixed
Content-Type: application/x-www-form-urlencoded
Events: duration=600

match=https://example.com/books/1&last_event_id=earliest

HTTP/1.1 200 OK
Content-Type: multipart/mixed; boundary=THIS_STRING_SEPARATES
Incremental: ?1
Events: duration=600
Accept-Query: application/x-www-form-urlencoded
Mercure-Last-Event-ID: earliest

--THIS_STRING_SEPARATES
Content-Event-Id: urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb
Content-Type: application/ld+json
Content-Length: 21

{"status": "shipped"}
--THIS_STRING_SEPARATES--
~~~

# Bounded Responses

Because a `multipart/mixed` response has no keep-alive mechanism, hubs **SHOULD** bound its
duration instead, and intermediaries' idle timeouts make long quiet responses fragile anyway.

The subscriber **MAY** request a bound with the `Events` request header field defined by
[@!I-D.gupta-httpapi-events-query], a Structured Fields Dictionary [@!RFC9651] whose `duration`
member is the requested response duration in seconds. The hub **MAY** honor a shorter duration
than requested — implementation-defined write timeouts and the expiration of the presented
access token both cap it — and **SHOULD** advertise the effective bound in the `Events`
response header field. A hub implementing this extension **SHOULD** honor the `Events` request
header field on `text/event-stream` responses too; hubs not implementing it ignore the field,
as its semantics are not part of [@!I-D.dunglas-mercure].

When the bound elapses, the hub **MUST** terminate the multipart body properly with its close
delimiter, distinguishing a completed response from a truncated one. The subscriber then
re-creates the subscription, resuming as described in (#resumption).

# Resumption

HTTP Events Query defines no resumption mechanism; the Mercure reconciliation model fills that
role unchanged. The subscriber **SHOULD** send the `Content-Event-Id` value of the last
received part — the update's `id`, verbatim — as its `last_event_id` subscription parameter
(or `Last-Event-ID` header field) when re-creating the subscription, and the hub applies the
state reconciliation rules of [@!I-D.dunglas-mercure], including the `earliest` reserved value
and the `Mercure-Last-Event-ID` response header.

# Binary Publication

[@!I-D.dunglas-mercure] requires publication requests to be encoded as
`application/x-www-form-urlencoded`, whose field values must be valid UTF-8: binary payloads
cannot be published losslessly. This document updates that requirement: a hub implementing
this extension **MUST** also accept publication requests encoded as `multipart/form-data`
[@!RFC7578].

The fields carry the same names and semantics as in [@!I-D.dunglas-mercure], with the
following differences:

*   The `data` part **MAY** carry any sequence of bytes; the UTF-8 requirement does not apply
    to it. All other field values remain subject to the constraints of
    [@!I-D.dunglas-mercure], including the UTF-8 and control-character rules on `topic`, `id`,
    and `type`.
*   The `data` part's `Content-Type` header field, if present, declares the media type
    [@!RFC9110] of the value. Hubs **MUST** reject values that are not valid media types with
    a 400 "Bad Request" HTTP status code, as a malformed value could otherwise inject metadata
    into subscription responses (see (#security-considerations)). The hub **MUST** convey the
    declared media type to subscribers when the response media type can carry per-event
    metadata (the `Content-Type` part header field of (#notification-encoding));
    `text/event-stream` defines no field for it, so it is not conveyed there.
*   A part without a field name has no `application/x-www-form-urlencoded` equivalent and
    **MUST** be ignored, like an unrecognized field.

A hub that does not implement this extension rejects `multipart/form-data` publication
requests with a 415 "Unsupported Media Type" HTTP status code, consistent with the base
requirement that publications be `application/x-www-form-urlencoded`.

When an update published as `multipart/form-data` is serialized as `text/event-stream`, the
hub **MUST** first encode its `data` value with base64 [@!RFC4648], whatever its content, and
emit the result as a single `data:` field. `text/event-stream` cannot carry arbitrary bytes,
and the format offers no per-event slot that could flag selective encoding: only a rule fixed
at publication time lets subscribers decode deterministically. Response media types able to
carry arbitrary bytes, such as the encoding of (#notification-encoding), **MUST** convey the
value verbatim.

Publishers that require byte-exact round-tripping (for example, of encrypted payloads)
**SHOULD** publish using `multipart/form-data`: the Server-Sent Events serialization of a
plain-form publication normalizes CR and CRLF in `data` to LF, while both the base64 encoding
above and the encoding of (#notification-encoding) preserve the exact bytes.

Example:

~~~ http
POST /.well-known/mercure HTTP/1.1
Host: example.com
Content-Type: multipart/form-data; boundary=THIS_STRING_SEPARATES
Authorization: Bearer [snip]

--THIS_STRING_SEPARATES
Content-Disposition: form-data; name="topic"

https://example.com/foo
--THIS_STRING_SEPARATES
Content-Disposition: form-data; name="data"
Content-Type: image/png

[binary bytes]
--THIS_STRING_SEPARATES--

HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8

urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb
~~~

# Discovery

A hub implementing this extension **SHOULD** advertise it with a `mercure_events_query` member
set to `true` in its protected resource metadata [@!RFC9728], alongside the members defined by
[@!I-D.dunglas-mercure]. Subscribers **MAY** also rely on the `Accept-Query` response header
field (see (#subscription-response-negotiation)) or simply attempt negotiation, since a hub
without the extension falls back to `text/event-stream`.

# Security Considerations

The security considerations of [@!I-D.dunglas-mercure] and [@!I-D.gupta-httpapi-events-query]
apply.

The field-injection reasoning of [@!I-D.dunglas-mercure] extends to this encoding:
publisher-supplied values **MUST NOT** be able to inject part framing or per-event metadata.
The random part boundary prevents payload bytes from forging a delimiter; the event ID
character constraints prevent header-field injection through `Content-Event-Id`; and the
media type validation of (#binary-publication) prevents it through `Content-Type` (CWE-93).

Binary publication removes the UTF-8 restriction on `data` only. Hubs **MUST NOT** relax the
character constraints on any other field, and the implementation-defined request body size
limits of [@!I-D.dunglas-mercure] apply to `multipart/form-data` requests identically —
per-part limits included, since a multipart body can contain many parts.

The base64 expansion of binary updates on `text/event-stream` responses increases the
serialized size by one third; hubs accounting for per-subscriber bandwidth **SHOULD** account
for the serialized size, not the payload size.

# IANA Considerations

This document has no IANA actions. The `Content-Event-Id` MIME extension field is used as
permitted by [@!RFC2045] without registration; a future revision of this document may register
it in the "Provisional Message Header Field Names" registry [@RFC3864].

{backmatter}

<reference anchor="HTML" target="https://html.spec.whatwg.org/review-drafts/2026-01/">
    <front>
        <title>HTML Living Standard (Review Draft, January 2026)</title>
        <author>
            <organization>The Web Hypertext Application Technology Working Group (WHATWG)</organization>
        </author>
        <date year="2026" month="January"/>
    </front>
</reference>
