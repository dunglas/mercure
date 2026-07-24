---
title: "GraphQL subscriptions over Mercure and SSE"
description: "Back GraphQL subscriptions with Mercure topics and Server-Sent Events instead of WebSockets, including an Apollo Client transport."
---

# GraphQL subscriptions

GraphQL subscriptions traditionally run over WebSockets ([`graphql-transport-ws`](https://github.com/enisdenjo/graphql-ws)). That works, but you end up with two real-time stacks if you also use Mercure for non-GraphQL push (HTML, agent state, notifications).

Mercure can carry GraphQL subscriptions directly. The pattern: the server returns a topic URL in response to a subscription query, and the client opens an `EventSource` on that topic.

## GraphQL subscriptions over Mercure: the flow

```text
# GraphQL Subscriptions over Mercure: The Flow
   client                          server
      |  POST /graphql              |
      |  subscription { msgAdded { ... } }
      | --------------------------> |
      |                             |
      |  { topic:                   |
      |      "https://example.com/  |
      |       graphql/subscriptions/abc123" }
      | <-------------------------- |
      |                             |
      |  GET /.well-known/mercure   |
      |     ?match=.../abc123       |
      | -----------------------------> hub
      |                             |
      |                             |  POST /publish (whenever the
      |                             |  data changes server-side)
      |                             | --------------> hub
      |  <--------------- SSE event ----------------|
```

The GraphQL server's job is reduced to:

1. Validate the subscription query.
2. Allocate a topic.
3. Return the topic URL.
4. Push payloads to that topic whenever the subscribed data changes.

The client subscribes to the topic with Mercure. When done, it closes the `EventSource`.

## Server-side GraphQL subscription resolver

A minimal Apollo Server resolver that returns a topic instead of starting a WebSocket subscription:

```javascript
// Server-Side GraphQL Subscription Resolver
const resolvers = {
  Subscription: {
    messageAdded: {
      // not the usual subscribe(), just resolve to a topic URL
      subscribe: (_root, { roomId }, ctx) => {
        const topic = `https://example.com/graphql/subscriptions/${roomId}/${ctx.user.id}`;
        return { topic };
      },
    },
  },
};
```

Wherever you mutate the data:

```javascript
// Server-Side GraphQL Subscription Resolver
async function postMessage(roomId, message) {
  await db.messages.insert({ roomId, ...message });
  for (const userId of await getMembers(roomId)) {
    await publish(
      `https://example.com/graphql/subscriptions/${roomId}/${userId}`,
      JSON.stringify({ data: { messageAdded: message } }),
      { private: true },
    );
  }
}
```

The payload should be the standard GraphQL response shape (`{ data, errors }`) so the client decoder can hand it straight to Apollo.

## Apollo client Mercure SSE transport

Apollo and other GraphQL clients accept a custom transport. Hand them an SSE-backed implementation:

```javascript
// Apollo Client Mercure SSE Transport
import {
  ApolloClient,
  InMemoryCache,
  split,
  HttpLink,
  Observable,
} from "@apollo/client";
import { getMainDefinition } from "@apollo/client/utilities";

const httpLink = new HttpLink({ uri: "/graphql" });

const sseLink = {
  request: ({ query, variables, operationName }) =>
    new Observable((observer) => {
      let es;
      const controller = new AbortController();

      // Ask the server for the subscription topic
      fetch("/graphql", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ query, variables, operationName }),
        signal: controller.signal,
      })
        .then((r) => r.json())
        .then(({ data: { topic } }) => {
          const url = new URL("https://hub.example.com/.well-known/mercure");
          url.searchParams.append("match", topic);
          es = new EventSource(url, { withCredentials: true });
          es.onmessage = (e) => observer.next(JSON.parse(e.data));
          es.onerror = (e) => observer.error(e);
        })
        .catch((err) => {
          if (err.name !== "AbortError") observer.error(err);
        });

      // Synchronous teardown: close the SSE if it opened, abort the fetch if it didn't.
      return () => {
        controller.abort();
        if (es) es.close();
      };
    }),
};

const link = split(
  ({ query }) => {
    const def = getMainDefinition(query);
    return (
      def.kind === "OperationDefinition" && def.operation === "subscription"
    );
  },
  sseLink,
  httpLink,
);

export const client = new ApolloClient({ link, cache: new InMemoryCache() });
```

The client uses HTTP for queries and mutations; subscriptions go through Mercure.

## Authorization

The same access token + cookie story as anywhere else. The server allocates topics that include the user's identity:

```text
# Authorization
https://example.com/graphql/subscriptions/<roomId>/<userId>
```

The user's access token covers `https://example.com/graphql/subscriptions/<roomId>/<their-user-id>` (and only that). Marking publications `private=on` ensures the hub enforces it.

For a subscriber to open one connection that covers all of their subscriptions across rooms:

```jsonc
// Authorization (header: { "alg": "...", "typ": "at+jwt" })
{
  "iss": "https://example.com",
  "aud": "https://hub.example.com/.well-known/mercure",
  "exp": 4102444800,
  "authorization_details": [
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["subscribe"],
      "topics": [
        {
          "match": "https://example.com/graphql/subscriptions/:room/42",
          "match_type": "urlpattern",
        },
      ],
    },
  ],
}
```

## Frameworks that already do this

- **API Platform.** [Built-in support for GraphQL subscriptions over Mercure](https://api-platform.com/docs/master/core/graphql/#subscriptions). Generate a Mercure topic per subscription and a working frontend, no glue code.
- **GraphQL Mesh, GraphQL Yoga.** Plugins exist; check the respective docs.

If your stack rolls its own GraphQL layer, the pattern in this guide is enough: a topic per subscription, a publish per data change, an `EventSource` on the client.

## When WebSockets are still better

- The subscription needs **client -> server messages on the subscription stream itself** (uncommon in GraphQL, but possible with `subscribe` operations that take live arguments).
- Latency budgets that make even `POST /graphql + GET /sub` round-trips a problem (rare; both run on HTTP/2 and the topic discovery is one extra request, once).

For everything else, Mercure plus GraphQL is a smaller stack: one transport for all real-time, no second port, no second protocol.

## Next steps for GraphQL subscriptions over Mercure

- [LLM token streaming](llm-token-streaming.md): for streaming responses outside of GraphQL.
- [Authorization](../concepts/authorization.md): per-user topics.
- [Active subscriptions](../concepts/active-subscriptions.md): knowing who's subscribed to a query.
