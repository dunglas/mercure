---
title: "GraphQL subscriptions over Mercure and SSE"
description: "Back GraphQL subscriptions with Mercure topics and Server-Sent Events instead of WebSockets, including an Apollo Client transport."
---

# GraphQL subscriptions with Mercure

Mercure can deliver GraphQL subscription results over SSE. Your GraphQL integration registers a subscription, allocates a topic, and publishes results when the selected data changes.

Topic registration is an application-defined operation. It is not a standard GraphQL response to a subscription request. [API Platform](https://api-platform.com/docs/core/graphql/#subscriptions) provides its own Mercure integration; the example below illustrates a custom integration.

## GraphQL subscriptions over Mercure: the flow

```text
   client                          server
      |  POST /graphql/subscriptions              |
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
      |                             |  POST update (whenever the
      |                             |  data changes server-side)
      |                             | --------------> hub
      |  <--------------- SSE event ----------------|
```

The server must:

1. Validate the subscription query.
2. Authorize access and allocate a topic.
3. Return the topic URL.
4. Execute the stored selection set with its variables and publish results when data changes.

The client opens a Mercure stream. The server must also expire registrations or provide an unsubscribe operation; closing an `EventSource` alone does not remove an application-level registration.

## Working Apollo Server example

This example uses Apollo Server 5 and a standard GraphQL subscription resolver. The resolver returns an `AsyncIterable`; a small Express endpoint registers the operation and forwards its executed results to Mercure. See [Apollo's subscription model](https://www.apollographql.com/docs/apollo-server/data/subscriptions).

Use Node.js 22 or later. In a new project, install the dependencies:

```console
npm install @apollo/server @as-integrations/express5 express graphql @graphql-tools/schema graphql-subscriptions
```

Set `MERCURE_URL` and `MERCURE_PUBLISHER_JWT` for a running hub. The publisher token must grant access to `https://example.com/graphql/subscriptions/*` using a URL Pattern. Enable anonymous subscriptions on this demo hub: **the example is a public message feed**, with no application login. Save this as `server.mjs`:

```javascript
import { randomUUID } from "node:crypto";
import express from "express";
import { ApolloServer } from "@apollo/server";
import { expressMiddleware } from "@as-integrations/express5";
import { makeExecutableSchema } from "@graphql-tools/schema";
import { getOperationAST, parse, subscribe, validate } from "graphql";
import { PubSub } from "graphql-subscriptions";

const { MERCURE_URL, MERCURE_PUBLISHER_JWT } = process.env;
if (!MERCURE_URL || !MERCURE_PUBLISHER_JWT) {
  throw new Error("Set MERCURE_URL and MERCURE_PUBLISHER_JWT");
}

const pubsub = new PubSub();
const schema = makeExecutableSchema({
  typeDefs: `
    type Message { id: ID!, text: String! }
    type Query { health: Boolean! }
    type Mutation { addMessage(text: String!): Message! }
    type Subscription { messageAdded: Message! }
  `,
  resolvers: {
    Query: { health: () => true },
    Mutation: {
      addMessage: async (_, { text }) => {
        const message = { id: randomUUID(), text };
        await pubsub.publish("MESSAGE_ADDED", { messageAdded: message });
        return message;
      },
    },
    Subscription: {
      messageAdded: {
        subscribe: () => pubsub.asyncIterableIterator("MESSAGE_ADDED"),
      },
    },
  },
});

async function publish(topic, result, type = "message") {
  const response = await fetch(MERCURE_URL, {
    method: "POST",
    headers: { Authorization: `Bearer ${MERCURE_PUBLISHER_JWT}` },
    body: new URLSearchParams({ topic, data: JSON.stringify(result), type }),
    signal: AbortSignal.timeout(10000),
  });
  if (!response.ok) throw new Error(`Publish failed: ${response.status}`);
}

const app = express();
app.use(express.json());
const apollo = new ApolloServer({ schema });
await apollo.start();
app.post("/graphql", expressMiddleware(apollo));

const registrations = new Map();
app.post("/graphql/subscriptions", async (req, res) => {
  const { query, variables, operationName } = req.body;
  let document;
  try {
    document = parse(query);
  } catch {
    return res.status(400).json({ error: "Invalid GraphQL document" });
  }
  const errors = validate(schema, document);
  if (errors.length) return res.status(400).json({ errors });
  if (getOperationAST(document, operationName)?.operation !== "subscription") {
    return res.status(400).json({ error: "Expected a subscription" });
  }

  const results = await subscribe({
    schema,
    document,
    variableValues: variables,
    operationName,
  });
  if (!(Symbol.asyncIterator in results)) {
    return res.status(400).json(results);
  }

  const id = randomUUID();
  const topic = `https://example.com/graphql/subscriptions/${id}`;
  const stop = () => {
    clearTimeout(timer);
    registrations.delete(id);
    return results.return();
  };
  const timer = setTimeout(() => void stop(), 5 * 60 * 1000);
  registrations.set(id, stop);

  async function forward() {
    try {
      for await (const result of results) await publish(topic, result);
      await publish(topic, {}, "complete");
    } catch (error) {
      console.error(error);
      await publish(topic, { errors: [{ message: "Subscription failed" }] });
    } finally {
      await stop();
    }
  }
  void forward().catch(console.error);
  res.json({ id, topic, lastEventId: "earliest" });
});

app.delete("/graphql/subscriptions/:id", async (req, res) => {
  await registrations.get(req.params.id)?.();
  res.sendStatus(204);
});

app.listen(4000, "127.0.0.1", () => {
  console.log("GraphQL ready at http://localhost:4000/graphql");
});
```

Run `node server.mjs`. Register a subscription:

```console
curl --fail-with-body http://localhost:4000/graphql/subscriptions \
  -H 'Content-Type: application/json' \
  --data '{"query":"subscription { messageAdded { text } }"}'
```

Open an `EventSource` on the returned topic, as in the client below. Then publish through an Apollo mutation:

```console
curl --fail-with-body http://localhost:4000/graphql \
  -H 'Content-Type: application/json' \
  --data '{"query":"mutation { addMessage(text: \"Hello from Apollo!\") { id } }"}'
```

Mercure delivers `{"data":{"messageAdded":{"text":"Hello from Apollo!"}}}`. GraphQL executes the stored selection set, so the subscription receives only the fields it requested. `lastEventId: "earliest"` covers results published before the client opens its stream.

Registrations expire after five minutes or when the client calls `DELETE`. This demo uses an in-memory `PubSub` and registration map in one application process. For production, authenticate registration and deletion, limit query cost and registrations, and use a shared event source for multiple application instances. Apply the [private-topic authorization](#authorization) below to confidential data.

## Apollo client Mercure SSE transport

For Apollo Client 4, install `@apollo/client`, `graphql`, and `rxjs`. This link uses the registration endpoint above. Serve or proxy `/graphql` and `/graphql/subscriptions` on the frontend's origin, and replace the hub URL with yours. Allow that origin in the hub's CORS configuration:

```javascript
import {
  ApolloClient,
  ApolloLink,
  HttpLink,
  InMemoryCache,
  split,
} from "@apollo/client";
import { getMainDefinition } from "@apollo/client/utilities";
import { print } from "graphql";
import { Observable } from "rxjs";

const httpLink = new HttpLink({ uri: "/graphql" });
const sseLink = new ApolloLink(
  ({ query, variables, operationName }) =>
    new Observable((observer) => {
      const controller = new AbortController();
      let es;
      let registrationId;
      const unregister = () => {
        if (!registrationId) return;
        void fetch(`/graphql/subscriptions/${registrationId}`, {
          method: "DELETE",
          keepalive: true,
        }).catch(console.error);
        registrationId = undefined;
      };

      fetch("/graphql/subscriptions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ query: print(query), variables, operationName }),
        signal: controller.signal,
      })
        .then(async (response) => {
          if (!response.ok)
            throw new Error(`Registration failed: ${response.status}`);
          return response.json();
        })
        .then(({ id, topic, lastEventId }) => {
          registrationId = id;
          if (controller.signal.aborted) return unregister();
          const url = new URL("https://hub.example.com/.well-known/mercure");
          url.searchParams.set("match", topic);
          if (lastEventId) url.searchParams.set("last_event_id", lastEventId);
          es = new EventSource(url, { withCredentials: true });
          es.onmessage = (event) => {
            try {
              observer.next(JSON.parse(event.data));
            } catch (error) {
              observer.error(error);
            }
          };
          es.addEventListener("complete", () => observer.complete());
          es.onerror = () => {
            if (es.readyState === EventSource.CLOSED) {
              observer.error(new Error("Subscription closed"));
            }
          };
        })
        .catch((error) => {
          if (!controller.signal.aborted) observer.error(error);
        });

      return () => {
        controller.abort();
        es?.close();
        unregister();
      };
    }),
);

const link = split(
  ({ query }) => {
    const definition = getMainDefinition(query);
    return (
      definition.kind === "OperationDefinition" &&
      definition.operation === "subscription"
    );
  },
  sseLink,
  httpLink,
);

export const client = new ApolloClient({ link, cache: new InMemoryCache() });
```

Queries and mutations use `/graphql`; subscriptions use the custom registration endpoint and Mercure. Transient SSE errors leave `EventSource` free to reconnect. The client unregisters when disposed, and server-side expiry covers abandoned registrations. The `complete` event ends an expired subscription; subscribe again if the UI still needs updates.

## Authorization

For private data, authenticate the registration request and authorize the selected resources. Issue a subscriber access token scoped to the allocated topic and send it with a cookie or bearer header. Add `private: "on"` to the server's publication fields. Authorize deletion against the registration's owner too.

The public example uses a random topic per registration. An application that groups private subscriptions by room and user can instead use:

```text
https://example.com/graphql/subscriptions/<roomId>/<userId>
```

The user's access token covers `https://example.com/graphql/subscriptions/<roomId>/<their-user-id>` (and only that). Marking publications `private=on` ensures the hub enforces it.

For a subscriber to open one connection that covers all of their subscriptions across rooms:

```json
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
          "match_type": "urlpattern"
        }
      ]
    }
  ]
}
```

## Frameworks that already do this

[API Platform supports GraphQL subscriptions through Mercure](https://api-platform.com/docs/core/graphql/#subscriptions). Follow its documented registration and authorization format rather than the custom format above.

## When WebSockets are still better

WebSockets fit applications that need frequent messages in both directions on the same channel. GraphQL subscriptions usually push results from server to client; queries and mutations already travel over HTTP. Mercure fits that model and adds topic authorization, replay, and a hub you can scale independently of your GraphQL servers.

Already using Mercure for notifications or AI streaming? Reuse it for GraphQL too. **[Mercure Cloud](https://mercure.rocks/pricing) operates the hub for you; [Mercure Enterprise](../production/high-availability.md) brings the same protocol to a supported cluster on your infrastructure.**

## Next steps

- [LLM token streaming](llm-token-streaming.md): for streaming responses outside of GraphQL.
- [Authorization](../concepts/authorization.md): per-user topics.
- [Active subscriptions](../concepts/active-subscriptions.md): knowing who's subscribed to a query.
