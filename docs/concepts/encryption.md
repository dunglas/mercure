---
title: "End-to-end encryption for Mercure updates with JWE"
description: "Encrypt update payloads with JSON Web Encryption so the Mercure hub itself cannot read them, with key distribution patterns."
---

# Mercure encryption

HTTPS protects data in transit, but a hub operator can read unencrypted payloads. Use [JSON Web Encryption](https://www.rfc-editor.org/rfc/rfc7516) when the hub must not have access to the content.

## How Mercure end-to-end encryption works

```text
publisher                    hub                       subscriber
    |                          |                           |
    |  encrypt(data, K)        |                           |
    | -----------------------> |  (sees ciphertext only)   |
    |                          | ------------------------> |  decrypt(ciphertext, K)
```

The publisher encrypts the `data` field before posting it. The hub stores and forwards the ciphertext like any other payload. The subscriber decrypts it after receiving the SSE event.

The hub never sees the plaintext, the key, or anything that lets it derive the key.

## Distributing the JWE key between publisher and subscriber

Choose a key-management scheme before encrypting updates. With symmetric encryption, publishers and subscribers share a secret. With the RSA example below, the publisher uses a public key and the subscriber holds the corresponding private key.

**1. Out-of-band.** The publisher and the subscriber both fetch the key from a side channel (your own API, a vault). The hub doesn't see the request.

**2. Through the discovery endpoint.** When the publisher controls the resource the subscriber fetches first, attach a `key-set` link to the discovery response:

```http
GET /books/1
Host: example.com
Authorization: Bearer <session token>

200 OK
Link: <https://hub.example.com/.well-known/mercure>; rel="mercure"
Link: <https://example.com/keys/books/1>; rel="key-set"
Content-Type: application/ld+json

{ "@id": "/books/1", "...": "..." }
```

The `key-set` URL serves a [JWK Set](https://www.rfc-editor.org/rfc/rfc7517). A public encryption key can be shared openly; symmetric keys and private decryption keys require an authenticated distribution channel. Never expose decryption keys to the hub.

## Publishing encrypted Mercure updates

```javascript
import { CompactEncrypt, importJWK } from "jose";

const key = await importJWK(publicJwk, "RSA-OAEP-256");
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
import { compactDecrypt, importJWK } from "jose";

const key = await importJWK(privateJwk, "RSA-OAEP-256");
const es = new EventSource(url);
es.onmessage = async (event) => {
  const { plaintext } = await compactDecrypt(event.data, key);
  const update = JSON.parse(new TextDecoder().decode(plaintext));
  // ...
};
```

The `jose` library handles JWE serialization as well as encryption. WebCrypto alone does not parse or produce JWE messages.

## What you give up

The debugger shows ciphertext. Decryption, key rotation, and recovery become application responsibilities. Mercure already matches on topics rather than payload contents, so encryption does not change topic matching.

Topics, event IDs, and event types remain visible to the hub. Opaque topic names can conceal resource names, but they do not hide traffic or SSE metadata.

## When to encrypt Mercure updates with JWE

Use JWE when your application requires content to remain unreadable to the hub operator, including in storage and logs. This applies to both managed and self-hosted hubs. Account for key distribution, revocation, and access to historical messages.

The examples assume `publicJwk` and `privateJwk` have been obtained through your application's key-management system. Do not send the private key with an update.

## Not a substitute for authorization

Encryption hides content; authorization controls who connects. You still need [the JWT layer](authorization.md) on top: to keep unauthorized clients off the hub, and to gate `private` updates so the hub knows who to deliver them to even when it can't read them.

For full data residency and operator control without the encryption overhead, [Self-Hosted Mercure](https://mercure.rocks/pricing) runs the same hub on your own infrastructure with no third party in the path.
