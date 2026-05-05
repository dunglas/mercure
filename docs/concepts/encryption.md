---
title: "End-to-end encryption for Mercure updates with JWE"
description: "Encrypt update payloads with JSON Web Encryption so the Mercure hub itself cannot read them, with key distribution patterns."
---

# Encryption

HTTPS protects data on the wire. It does not protect data **from the hub**. Operators of a Mercure hub can read every update that flows through it. For most cases that's fine; the hub is your infrastructure.

When it isn't (third-party hub, multi-tenant hub, regulated data), encrypt the payload end-to-end with [JSON Web Encryption](https://www.rfc-editor.org/rfc/rfc7516).

## How Mercure end-to-end encryption works

```text
# How Mercure End-to-End Encryption Works
publisher                    hub                       subscriber
    |                          |                           |
    |  encrypt(data, K)        |                           |
    | -----------------------> |  (sees ciphertext only)   |
    |                          | ------------------------> |  decrypt(ciphertext, K)
```

The publisher encrypts the `data` field before posting it. The hub stores and forwards the ciphertext like any other payload. The subscriber decrypts it after receiving the SSE event.

The hub never sees the plaintext, the key, or anything that lets it derive the key.

## Distributing the JWE key between publisher and subscriber

The publisher and the subscriber need a shared key. Two patterns work:

**1. Out-of-band.** The publisher and the subscriber both fetch the key from a side channel (your own API, a vault). The hub doesn't see the request.

**2. Through the discovery endpoint.** When the publisher controls the resource the subscriber fetches first, attach a `key-set` link to the discovery response:

```http
# Distributing the JWE Key Between Publisher and Subscriber
GET /books/1 HTTP/1.1
Host: example.com
Authorization: Bearer <session token>

200 OK
Link: <https://hub.example.com/.well-known/mercure>; rel="mercure"
Link: <https://example.com/keys/books/1>; rel="key-set"
Content-Type: application/ld+json

{ "@id": "/books/1", "...": "..." }
```

The `key-set` URL must serve [JWK Set](https://www.rfc-editor.org/rfc/rfc7517) JSON. **Authorize the request.** Anyone who reads the key set can decrypt the updates. Reuse the same auth your application already enforces.

## Publishing encrypted Mercure updates

```javascript
// Publishing Encrypted Mercure Updates
import { CompactEncrypt } from "jose";

const key = /* the JWK */;
const plaintext = JSON.stringify({ status: "checked out" });

const jwe = await new CompactEncrypt(new TextEncoder().encode(plaintext))
  .setProtectedHeader({ alg: "RSA-OAEP-256", enc: "A256GCM" })
  .encrypt(key);

await fetch("https://hub.example.com/.well-known/mercure", {
  method: "POST",
  headers: {
    Authorization: `Bearer ${jwt}`,
    "Content-Type": "application/x-www-form-urlencoded",
  },
  body: new URLSearchParams({
    topic: "https://example.com/books/1",
    data: jwe,
  }),
});
```

## Decrypting on the subscriber

In a browser, with [`jose`](https://github.com/panva/jose):

```javascript
// Decrypting on the subscriber
import { compactDecrypt } from "jose";

const key = await fetchKey(); // from the key-set link
const es = new EventSource(url);
es.onmessage = async (event) => {
  const { plaintext } = await compactDecrypt(event.data, key);
  const update = JSON.parse(new TextDecoder().decode(plaintext));
  // ...
};
```

The browser's WebCrypto API can do the same without an external library if you only need a fixed algorithm.

## What you give up

End-to-end encryption rules out a few hub-side conveniences:

- **Server-side filtering on payload contents.** The hub can't introspect the data, so any matching has to be on the topic.
- **Server-rendered debug UI.** The demo UI shows ciphertext, not plaintext.
- **Partial updates that depend on aggregating prior state at the hub.** Any aggregation has to happen on the subscriber.

The topic and the SSE metadata (event ID, type) are still visible to the hub. If you need to hide _those_ too, derive opaque topics from a secret and rotate them.

## When to encrypt Mercure updates with JWE

End-to-end encryption is the right call when:

- The hub is operated by a different org than the publisher (multi-tenant SaaS, third-party deployment).
- The data is subject to regulation (GDPR sensitive categories, HIPAA, financial data) and you can't argue that the hub processor is in scope.
- You're moving to [Cloud](https://mercure.rocks/pricing) but a subset of topics carries data you'd rather not have us see.

If the hub is yours and you control its hosts, HTTPS is sufficient and JWE adds operational cost without a meaningful security gain.

## Not a substitute for authorization

Encryption hides content; authorization controls who connects. You still need [the JWT layer](authorization.md) on top: to keep unauthorized clients off the hub, and to gate `private` updates so the hub knows who to deliver them to even when it can't read them.

For full data residency and operator control without the encryption overhead, [Self-Hosted Mercure](https://mercure.rocks/pricing) runs the same hub on your own infrastructure with no third party in the path.
