# GraphQL Subscriptions

GraphQL subscriptions traditionally run over WebSockets ([`graphql-transport-ws`](https://github.com/enisdenjo/graphql-ws)). That works, but you end up with two real-time stacks if you also use Mercure for non-GraphQL push (HTML, agent state, notifications).

Mercure can carry GraphQL subscriptions directly. The pattern: the server returns a topic URL in response to a subscription query, and the client opens an `EventSource` on that topic.

## The flow

```
   client                          server
      │  POST /graphql              │
      │  subscription { msgAdded { ... } }
      │ ──────────────────────────► │
      │                             │
      │  { topic:                   │
      │      "https://example.com/  │
      │       graphql/subscriptions/abc123" }
      │ ◄────────────────────────── │
      │                             │
      │  GET /.well-known/mercure   │
      │     ?match=…/abc123         │
      │ ──────────────────────────────► hub
      │                             │
      │                             │  POST /publish (whenever the
      │                             │  data changes server-side)
      │                             │ ──────────────► hub
      │  ◄─────────────── SSE event ─────────────────│
```

The GraphQL server's job is reduced to:

1. Validate the subscription query.
2. Allocate a topic.
3. Return the topic URL.
4. Push payloads to that topic whenever the subscribed data changes.

The client subscribes to the topic with Mercure. When done, it closes the `EventSource`.

## Server side

A minimal Apollo Server resolver that returns a topic instead of starting a WebSocket subscription:

```javascript
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

## Client side

Apollo and other GraphQL clients accept a custom transport. Hand them an SSE-backed implementation:

```javascript
import { ApolloClient, InMemoryCache, split, HttpLink } from "@apollo/client";
import { getMainDefinition } from "@apollo/client/utilities";

const httpLink = new HttpLink({ uri: "/graphql" });

const sseLink = {
  request: ({ query, variables, operationName }) =>
    new Observable((observer) => {
      // Ask the server for the subscription topic
      fetch("/graphql", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ query, variables, operationName }),
      })
        .then((r) => r.json())
        .then(({ data: { topic } }) => {
          const url = new URL("https://hub.example.com/.well-known/mercure");
          url.searchParams.append("match", topic);
          const es = new EventSource(url, { withCredentials: true });
          es.onmessage = (e) => observer.next(JSON.parse(e.data));
          es.onerror = (e) => observer.error(e);
          return () => es.close();
        });
    }),
};

const link = split(
  ({ query }) => {
    const def = getMainDefinition(query);
    return def.kind === "OperationDefinition" && def.operation === "subscription";
  },
  sseLink,
  httpLink,
);

export const client = new ApolloClient({ link, cache: new InMemoryCache() });
```

The client uses HTTP for queries and mutations; subscriptions go through Mercure.

## Authorization

The same JWT + cookie story as anywhere else. The server allocates topics that include the user's identity:

```
https://example.com/graphql/subscriptions/<roomId>/<userId>
```

The user's JWT covers `https://example.com/graphql/subscriptions/<roomId>/<their-user-id>` (and only that). Marking publications `private=on` ensures the hub enforces it.

For a subscriber to open one connection that covers all of their subscriptions across rooms:

```json
{
  "mercure": {
    "subscribe": [
      {
        "match": "https://example.com/graphql/subscriptions/:room/42",
        "matchType": "URLPattern"
      }
    ]
  }
}
```

## Frameworks that already do this

- **API Platform.** [Built-in support for GraphQL subscriptions over Mercure](https://api-platform.com/docs/master/core/graphql/#subscriptions). Generate a Mercure topic per subscription and a working frontend, no glue code.
- **GraphQL Mesh, GraphQL Yoga.** Plugins exist; check the respective docs.

If your stack rolls its own GraphQL layer, the pattern in this guide is enough — a topic per subscription, a publish per data change, an `EventSource` on the client.

## When WebSockets are still better

- The subscription needs **client → server messages on the subscription stream itself** (uncommon in GraphQL, but possible with `subscribe` operations that take live arguments).
- Latency budgets that make even `POST /graphql + GET /sub` round-trips a problem (rare; both run on HTTP/2 and the topic discovery is one extra request, once).

For everything else, Mercure plus GraphQL is a smaller stack — one transport for all real-time, no second port, no second protocol.

## Next

- [LLM token streaming](llm-token-streaming.md) — for streaming responses outside of GraphQL.
- [Authorization](../concepts/authorization.md) — per-user topics.
- [Active subscriptions](../concepts/active-subscriptions.md) — knowing who's subscribed to a query.
