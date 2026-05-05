# LLM Token Streaming

LLMs return tokens one at a time. To deliver that to a browser as it happens, you need a streaming transport between your server and the browser. Mercure is a good fit for this: it's already SSE under the hood, no WebSocket gateway, no proprietary client.

This guide builds a minimal server-side LLM endpoint that calls OpenAI's streaming API and forwards tokens to the browser through Mercure. The same shape works with Anthropic, Google, Mistral, AWS Bedrock, vLLM, llama.cpp — anything with a streaming API.

## Architecture

```
                   ┌──────────┐  POST /chat       ┌────────────┐
   browser ───────►│  origin  │─────────────────► │            │
        ▲          │  server  │                   │  Mercure   │
        │          │          │  POST /publish    │    hub     │
        │          │          │─────────────────► │            │
        │          └──────────┘                   │            │
        │                                         │            │
        └─────────────GET /.well-known/mercure────┤            │
                       (SSE, with cookie)         └────────────┘

                    server stream loop:
                    ─────────────────────
                    for delta in openai.stream(prompt):
                        publish(topic="conv:42", data=delta)
                    publish(topic="conv:42", type="done")
```

Why this works:

- The browser holds **one** `EventSource` connection to the hub. It receives every token of every chat turn over that connection, regardless of how many models or backends you use.
- The origin server doesn't have to keep the browser's HTTP connection open. It calls the hub and goes back to its event loop. This makes serverless inference (Lambda, Cloud Run, Workers) trivial.
- Reconnection is built in. If the browser drops mid-stream, `EventSource` reconnects with `Last-Event-ID` and the hub resends any tokens it still has buffered.

## Subscriber: the browser

Open one connection per chat session. Use a topic that includes the conversation ID:

```html
<div id="output"></div>

<script type="module">
  const conversationId = "42";

  const url = new URL("https://hub.example.com/.well-known/mercure");
  url.searchParams.append("match", `https://example.com/conversations/${conversationId}`);

  const es = new EventSource(url, { withCredentials: true });
  const out = document.getElementById("output");

  es.addEventListener("token", (e) => {
    out.append(JSON.parse(e.data).text);
  });

  es.addEventListener("done", () => {
    es.close();
  });
</script>
```

The cookie carries a JWT whose `mercure.subscribe` claim authorizes the user for `https://example.com/conversations/<their-id>` topics only. See [Authorization](../concepts/authorization.md).

## Publisher: server-side OpenAI streaming

A Node.js handler that calls OpenAI's chat completions in streaming mode and forwards each delta to the hub:

```javascript
import OpenAI from "openai";

const openai = new OpenAI();
const HUB = "https://hub.example.com/.well-known/mercure";
const PUBLISHER_JWT = process.env.MERCURE_PUBLISHER_JWT;

async function publish(topic, data, type = "message") {
  await fetch(HUB, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${PUBLISHER_JWT}`,
      "Content-Type": "application/x-www-form-urlencoded",
    },
    body: new URLSearchParams({ topic, data, type }),
  });
}

export async function streamCompletion(conversationId, prompt) {
  const topic = `https://example.com/conversations/${conversationId}`;

  const stream = await openai.chat.completions.create({
    model: "gpt-4o",
    messages: [{ role: "user", content: prompt }],
    stream: true,
  });

  for await (const chunk of stream) {
    const text = chunk.choices[0]?.delta?.content;
    if (text) {
      await publish(topic, JSON.stringify({ text }), "token");
    }
  }

  await publish(topic, "{}", "done");
}
```

The publish call is a single `POST`; it returns as soon as the hub accepts the update. You don't await delivery to subscribers — that happens in the background.

## Why not just stream the response from the origin?

`POST /chat` could return `text/event-stream` directly. That works too, but it has tradeoffs Mercure avoids:

- **Connection stickiness.** A streaming response keeps the origin worker tied to that one client until the stream finishes. With Mercure, the origin worker publishes and exits; the hub holds the long connection.
- **Multi-tab.** With Mercure, a user with three tabs open sees the same stream in all three. With direct streaming, you'd have to fan it out yourself.
- **Disconnect resilience.** Mercure re-delivers tokens after a reconnect from the buffer. A direct stream doesn't — the user reloads and the stream is gone.
- **Multi-model fan-out.** If you want to stream from several models in parallel, run two prompts at once, or push a tool result mid-stream, separate publishes are easier than splitting a single response stream.

That said, for the simplest case (one model, one tab, one prompt), a streaming HTTP response from your origin is fine. Reach for Mercure when the simple case stops being enough.

## Other providers

The pattern is identical — replace the streaming call.

**Anthropic:**

```javascript
import Anthropic from "@anthropic-ai/sdk";
const client = new Anthropic();

const stream = client.messages.stream({
  model: "claude-sonnet-4-6",
  max_tokens: 1024,
  messages: [{ role: "user", content: prompt }],
});

for await (const event of stream) {
  if (event.type === "content_block_delta" && event.delta.type === "text_delta") {
    await publish(topic, JSON.stringify({ text: event.delta.text }), "token");
  }
}
```

**Local model (vLLM, llama.cpp, Ollama):** any of these expose an OpenAI-compatible streaming endpoint. Point the OpenAI client at it (`baseURL`) and the code above works unchanged.

**Bedrock, Vertex, etc.:** the streaming API has a different shape, but the structure (iterate, publish per delta) is the same.

## Performance notes

- **Don't await each publish if latency matters.** Fire-and-forget the `fetch` calls and `await Promise.all` at the end. Each call is a few hundred bytes; serializing them adds 1–5ms per token.
- **Batch tiny tokens.** OpenAI sometimes emits single-character deltas. If your UI renders per token, that's fine; if you're hitting publish-rate limits on a managed hub, accumulate 50–100ms worth of tokens and publish in chunks.
- **Stream IDs help replay.** Set a custom `id=` on each publish (a counter, or `<conversationId>:<index>`) so a reconnecting client can ask for everything after the last one it saw.

## Authorization sketch

Mint a JWT for the user when they load the chat page:

```json
{
  "mercure": {
    "subscribe": [
      {
        "match": "https://example.com/conversations/42",
        "payload": { "user": "alice" }
      }
    ]
  },
  "exp": 1730000000
}
```

Set it as the `mercureAuthorization` cookie with `Domain=example.com; Path=/.well-known/mercure; Secure; HttpOnly; SameSite=Strict`. The browser's `EventSource` picks it up automatically. See [Authorization](../concepts/authorization.md).

## Limits to be aware of

- **One `EventSource` per browser tab is enough.** A tab can have one connection per origin under HTTP/2 and use `match*` parameters for as many topics as it wants.
- **Connection counts.** A streaming chat keeps a connection open for the life of the page. The open-source hub has [no built-in cap](../concepts/reconnection-and-history.md#the-history-buffer) — sizing is whatever your hardware can handle. Cloud tiers cap connections per plan.
- **Buffer size.** If you want a user reloading mid-stream to recover the in-progress answer, set the hub's history buffer high enough to cover a typical answer's worth of tokens (5,000+).

> **Pro tip.** For prototyping a streaming chat UI without provisioning infrastructure, the [Mercure Cloud Free tier](https://mercure.rocks/pricing) gives you a hub in seconds. Move to self-hosted later if connection volume or compliance demands it — the protocol is identical.

## A complete reference

The pattern in this guide drives the SSE side of [Anthropic's web search streaming demos](https://github.com/anthropics) and several production chatbots built on the [API Platform framework](https://api-platform.com/docs/core/mercure/). [Awesome Mercure](../ecosystem/awesome.md) has more examples.

## Next

- [AI agent progress](ai-agent-progress.md) — when there's more than tokens to stream.
- [Authorization](../concepts/authorization.md) — minting per-conversation tokens.
- [Reconnection and history](../concepts/reconnection-and-history.md) — surviving a mid-stream disconnect.
