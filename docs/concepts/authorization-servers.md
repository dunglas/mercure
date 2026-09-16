---
title: "Using an OAuth 2.0 authorization server with Mercure"
description: "Issue Mercure access tokens from Keycloak or any OAuth 2.0 authorization server that can mint RFC 9396 authorization_details claims, and point the hub at its JWKS."
---

# Authorization servers

The hub is an OAuth 2.0 protected resource: it validates access tokens, it never issues them. Small applications mint their own tokens with a shared secret ([Authorization](authorization.md)). Once several applications publish to the same hub, or users log in through an identity provider, the tokens should come from a real **authorization server** instead.

Any authorization server works, as long as it can put an [RFC 9396](https://www.rfc-editor.org/rfc/rfc9396) `authorization_details` claim in a JWT access token. This page covers what the hub expects from it, and walks through [Keycloak](#keycloak) as a worked example.

## What the authorization server must produce

The hub accepts a token when all of the following hold. Most are plain [RFC 9068](https://www.rfc-editor.org/rfc/rfc9068) requirements that any compliant server already satisfies.

| Requirement             | Value                                                                                                    |
| ----------------------- | -------------------------------------------------------------------------------------------------------- |
| `typ` header            | `at+jwt`. Servers that default to `JWT` need to be told; the hub rejects anything else.                  |
| `iss` claim             | Exactly the identifier configured in the hub's `issuer` block.                                           |
| `aud` claim             | Contains the hub's resource identifier. Extra audiences are fine.                                        |
| `exp` claim             | Required. Keep it short: minutes, not days.                                                              |
| Signature               | An asymmetric algorithm whose public key is published in a JWK Set. The hub only ever holds public keys. |
| `authorization_details` | One or more entries of type `https://mercure.rocks/authorization-detail`.                                |

A Mercure authorization detail grants a set of actions over a set of [topic matchers](topics-and-matchers.md):

```jsonc
// A Mercure authorization detail
{
  "type": "https://mercure.rocks/authorization-detail",
  "actions": ["subscribe"],
  "topics": [
    { "match": "https://example.com/users/42/notifications" },
    { "match": "https://example.com/books/:id", "match_type": "urlpattern" },
  ],
  "payload": { "user": "https://example.com/users/42" },
}
```

[Authorization](authorization.md#authorization-details) describes each member. Entries with another `type` are ignored, so one token can carry grants for several resources; one malformed Mercure entry rejects the whole token.

## Pointing the hub at the authorization server

Declare the server as a trusted `issuer` and verify its signatures against its JWK Set:

```caddyfile
# Pointing the hub at the authorization server
mercure {
  issuer https://auth.example.com/realms/main {
    authorization_server

    publisher {
      jwks_uri https://auth.example.com/realms/main/protocol/openid-connect/certs RS256
    }
    subscriber {
      jwks_uri https://auth.example.com/realms/main/protocol/openid-connect/certs RS256
    }
  }
}
```

Three things to note:

- The issuer identifier is matched against `iss` **by exact string comparison**. It is the server's public URL, even when the hub reaches it on a private address: `jwks_uri` is the address the hub dials, and the two need not be the same.
- Keys are bound to their issuer and never pooled across issuers, so a token from one server is never verified with another's keys. Add one `issuer` block per server.
- The algorithm comes from the configuration, never from the token header. Pin it (`RS256` above) to block algorithm-confusion attacks.

`authorization_server` advertises the issuer in the hub's [RFC 9728](https://www.rfc-editor.org/rfc/rfc9728) protected resource metadata, which is how a generic OAuth 2.0 client discovers where to get a token:

```console
# Reading the protected resource metadata
curl https://hub.example.com/.well-known/oauth-protected-resource/.well-known/mercure
```

```json
{
  "resource": "https://hub.example.com/.well-known/mercure",
  "bearer_methods_supported": ["header"],
  "authorization_servers": ["https://auth.example.com/realms/main"],
  "authorization_details_types_supported": [
    "https://mercure.rocks/authorization-detail"
  ],
  "mercure_cookie": "__Secure-mercure_access_token"
}
```

See [Discovery](discovery.md) for the client-side, and [Configuration](../deployment/configuration.md#jwt-validation-via-jwks) for the full directive reference.

## Two ways the claim gets into the token

The hub does not care how `authorization_details` was produced, only that the server signed it. There are two ways to get there.

**Natively.** A server implementing RFC 9396 accepts an `authorization_details` parameter on the authorization or token request, validates it against what the client and user are actually allowed, and echoes the approved set back in the token. Clients ask for what they need and get a subset. This is the model the protocol was designed for, and it needs no Mercure-specific server configuration beyond registering the detail type. Among open-source servers, [node-oidc-provider](https://github.com/panva/node-oidc-provider) supports RFC 9396 out of the box, and already stamps the `at+jwt` header the hub requires.

**Through a claim mapper.** Most deployed servers do not implement RFC 9396 yet, but nearly all can inject a custom claim into an access token: a mapper, hook, action, or rule, depending on the vendor. Store each subject's grants alongside the account and map them into `authorization_details` at token time. The token is identical on the wire; only the negotiation is missing.

The rest of this page does the second, because it is the case that needs explaining.

## Keycloak

Keycloak has no native Rich Authorization Requests support, so the grants live in a user attribute and a protocol mapper copies them into the claim. Four pieces of configuration.

### The `at+jwt` header type

Keycloak stamps `typ: JWT` on access tokens unless the client opts in, and the hub rejects those. Turn it on per client, under **Clients → _your client_ → Advanced → Fine grain OpenID Connect configuration → Use "at+jwt" as access token header type**, or in a realm export:

```json
"attributes": { "access.token.header.type.rfc9068": "true" }
```

### The audience

Add an **Audience** mapper whose included custom audience is the hub's resource identifier:

```json
{
  "name": "mercure-audience",
  "protocol": "openid-connect",
  "protocolMapper": "oidc-audience-mapper",
  "config": {
    "included.custom.audience": "https://hub.example.com/.well-known/mercure",
    "access.token.claim": "true"
  }
}
```

Keycloak adds audiences of its own; the hub only requires that its identifier be among them.

### Per-user grants

Store the grants in a user attribute named `authorization_details`, then map it with a **User Attribute** mapper. The important setting is `jsonType.label`: with `JSON`, Keycloak parses the stored attribute and emits a real array instead of a string, which is what the hub needs.

```json
{
  "name": "mercure-authorization-details",
  "protocol": "openid-connect",
  "protocolMapper": "oidc-usermodel-attribute-mapper",
  "config": {
    "user.attribute": "authorization_details",
    "claim.name": "authorization_details",
    "jsonType.label": "JSON",
    "access.token.claim": "true",
    "multivalued": "false"
  }
}
```

Declare `authorization_details` in the realm's user profile with `"permissions": {"view": ["admin"], "edit": ["admin"]}`. Users must never be able to edit their own grants, and unmanaged attributes are dropped unless the realm allows them.

The attribute holds the JSON array verbatim:

```json
[
  {
    "type": "https://mercure.rocks/authorization-detail",
    "actions": ["subscribe"],
    "topics": [
      { "match": "https://example.com/users/42/notifications" },
      { "match": "https://example.com/announcements" }
    ],
    "payload": { "user": "https://example.com/users/42" }
  }
]
```

### Publisher grants

A backend that publishes on behalf of the application authenticates with `client_credentials` and always carries the same grants, so a **Hardcoded claim** mapper on that client is enough:

```json
{
  "name": "mercure-authorization-details",
  "protocol": "openid-connect",
  "protocolMapper": "oidc-hardcoded-claim-mapper",
  "config": {
    "claim.name": "authorization_details",
    "claim.value": "[{\"type\":\"https://mercure.rocks/authorization-detail\",\"actions\":[\"publish\"],\"topics\":[{\"match\":\"https://example.com/users/:user/notifications\",\"match_type\":\"urlpattern\"}]}]",
    "jsonType.label": "JSON",
    "access.token.claim": "true"
  }
}
```

Keep publisher grants as narrow as the service actually needs. A URL Pattern covering one route is better than `*`.

### Checking the result

Decode an issued token and confirm the header and the claim:

```console
# Checking the result
curl -s -X POST https://auth.example.com/realms/main/protocol/openid-connect/token \
  -d grant_type=client_credentials \
  -d client_id=publisher -d client_secret="$SECRET" |
  jq -r .access_token | cut -d. -f1,2 | tr '.' '\n' | base64 -d 2>/dev/null | jq
```

The header must read `"typ": "at+jwt"`, and `authorization_details` must be a JSON array, not a string. A quoted string is the usual symptom of a missing `jsonType.label`.

A complete runnable setup, with a hub, a Keycloak realm and a small application wired together in Docker Compose, is available at [dunglas/demo-mercure-keycloak](https://github.com/dunglas/demo-mercure-keycloak).

### Letting Keycloak clients narrow their own tokens

A claim mapper gives every token of a subject the same grants. To get the negotiation RFC 9396 describes, where a client asks for a subset of what it may have, Keycloak needs a provider: [dunglas/keycloak-rar](https://github.com/dunglas/keycloak-rar) implements one for the Mercure detail type.

It exists because Keycloak validates an `authorization_details` request but writes the result to the token endpoint's JSON response body, never to the access token, so the claim never reaches the hub on its own. The provider pairs the validation with a protocol mapper that puts the approved set in the token.

With it, a client that only needs one topic for one session asks for it, and the token it gets is worth nothing more:

```console
# Letting Keycloak clients narrow their own tokens
curl -X POST https://auth.example.com/realms/main/protocol/openid-connect/token \
  -d grant_type=client_credentials -d client_id=publisher -d client_secret="$SECRET" \
  --data-urlencode 'authorization_details=[{"type":"https://mercure.rocks/authorization-detail","actions":["publish"],"topics":[{"match":"https://example.com/books/1"}]}]'
```

Requesting more than the subject is granted is refused with `400 invalid_authorization_details` on the authorization code grant. Read that project's notes before deploying it: it builds on a private Keycloak SPI that changes between minor releases.

## Letting clients ask for less

When the server implements RFC 9396 natively, a client requests only the grants it needs for the session:

```http
# Letting clients ask for less
POST /token HTTP/1.1
Host: auth.example.com
Content-Type: application/x-www-form-urlencoded

grant_type=client_credentials&authorization_details=%5B%7B%22type%22%3A%22https%3A%2F%2Fmercure.rocks%2Fauthorization-detail%22%2C%22actions%22%3A%5B%22publish%22%5D%2C%22topics%22%3A%5B%7B%22match%22%3A%22https%3A%2F%2Fexample.com%2Fbooks%2F1%22%7D%5D%7D%5D
```

The server returns the approved subset in the token. With a claim mapper, the same narrowing is done by the application instead: request a token per scope of work, or keep separate clients for separate jobs.

## Security checklist

- **One `issuer` block per authorization server.** Never reuse one block for two servers; the hub would then accept either server's keys for either issuer.
- **Pin the algorithms.** `jwks_uri <url> RS256` rather than the default allowlist when you know what the server signs with.
- **Keep `exp` short.** The hub drops the connection when the token expires and the browser reconnects, so short lifetimes cost little. Refresh the token and update the cookie before it expires.
- **Grants belong to the server, not the client.** Whatever stores them (a user attribute, a database, a policy engine) must be writable only by administrators.
- **HTTPS everywhere.** The hub refuses tokens over plain HTTP for any non-anonymous request.

## Troubleshooting

| Symptom                                                | Cause                                                                                          |
| ------------------------------------------------------ | ---------------------------------------------------------------------------------------------- |
| `401 invalid_token`, token looks fine                  | `typ` is `JWT` instead of `at+jwt`, or `iss` does not match the `issuer` block byte for byte   |
| `401 invalid_token` right after switching servers      | The hub is verifying with the other issuer's keys: each server needs its own block             |
| `401 invalid_token`, `authorization_details` present   | The claim is a JSON string rather than an array, or a `topics` entry is a bare string          |
| `403 insufficient_scope` on publish                    | No entry grants `publish` on that topic; alternate topics each need their own grant            |
| Subscriber connects but never receives private updates | No entry grants `subscribe` on the update's topic                                              |
| Hub logs a JWKS fetch failure                          | `jwks_uri` is unreachable from the hub, which may need the private address, not the public one |

[Troubleshooting](../production/troubleshooting.md) covers the general cases.
