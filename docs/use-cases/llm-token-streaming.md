---
title: "Stream LLM tokens to the browser with Mercure and SSE"
description: "Stream OpenAI, Anthropic, or local-model tokens to the browser in real time using Mercure and Server-Sent Events without a WebSocket gateway."
---

# Stream LLM tokens with Mercure

A streaming model API returns chunks of output as generation progresses. Your server can publish those chunks to Mercure so authorized browsers receive them over SSE.

The example uses the [OpenAI Node.js SDK](https://github.com/openai/openai-node) and a Mercure publisher token. Set `OPENAI_API_KEY`, `OPENAI_MODEL` to a Chat Completions-compatible model available to your account, and `MERCURE_PUBLISHER_JWT`. The hub and application must also configure the subscriber cookie and CORS.

## LLM token streaming architecture with Mercure

```text
browser -- POST /chat --> application -- enqueue --> generation worker
browser -- SSE subscription ------------------------------> Mercure hub
generation worker -- model request --> model API
generation worker -- POST /.well-known/mercure -----------> Mercure hub
browser <---------------------- output chunks ------------- Mercure hub
```

The worker stays active while the model generates output. To return the browser's initiating request early, schedule the work in a queue or a runtime that supports background tasks. Closing the browser does not cancel that worker.

The browser can replay retained chunks after a disconnect. The example requests `earliest` on a response-specific topic to cover output published before it subscribes.

## Subscriber: the browser

Use one topic per generated response. The example uses conversation `42` and response `1`; your application must allocate these IDs and authorize the user before starting the worker.

```html
<div id="output"></div>

<script type="module">
  const conversationId = "42";
  const responseId = "1";

  const url = new URL("https://hub.example.com/.well-known/mercure");
  url.searchParams.append(
    "match",
    `https://example.com/conversations/${conversationId}/responses/${responseId}`,
  );

  url.searchParams.set("last_event_id", "earliest");
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

The cookie carries an OAuth 2.0 access token whose `subscribe` grant authorizes the user for `https://example.com/conversations/<their-id>/responses/<response-id>` topics only. See [Authorization](../concepts/authorization.md).

## Publisher: server-side OpenAI streaming

A worker function using [Chat Completions streaming](https://developers.openai.com/api/reference/resources/chat):

```javascript
import OpenAI from "openai";

const openai = new OpenAI();
const HUB = "https://hub.example.com/.well-known/mercure";
const PUBLISHER_JWT = process.env.MERCURE_PUBLISHER_JWT;

async function publish(topic, data, type = "message") {
  const response = await fetch(HUB, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${PUBLISHER_JWT}`,
      "Content-Type": "application/x-www-form-urlencoded",
    },
    body: new URLSearchParams({ topic, data, type, private: "on" }),
  });
  if (!response.ok)
    throw new Error(`Mercure publish failed: ${response.status}`);
}

export async function streamCompletion(conversationId, responseId, prompt) {
  const topic = `https://example.com/conversations/${conversationId}/responses/${responseId}`;

  const stream = await openai.chat.completions.create({
    model: process.env.OPENAI_MODEL,
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

Each publish waits for the hub's response and checks its status. This preserves chunk order. Acceptance does not acknowledge that subscribers have processed the update.

## Why not just stream the response from the origin?

Returning an SSE response directly is sufficient for one client consuming one generation. Use Mercure when several tabs or devices need the same stream, when output comes from background workers, or when you need retained-event replay.

Mercure holds subscriber connections, but the generation worker still runs until the model finishes. Serverless execution limits apply to that worker.

## Other LLM providers with Mercure

The pattern is identical. Replace the streaming call.

**Anthropic:** set `ANTHROPIC_API_KEY` and `ANTHROPIC_MODEL` to a model that supports the [Messages streaming API](https://platform.claude.com/docs/en/build-with-claude/streaming). This fragment reuses the `publish` helper above and assumes `prompt` and `topic` are set.

```javascript
import Anthropic from "@anthropic-ai/sdk";
const client = new Anthropic();

const stream = client.messages.stream({
  model: process.env.ANTHROPIC_MODEL,
  max_tokens: 1024,
  messages: [{ role: "user", content: prompt }],
});

for await (const event of stream) {
  if (
    event.type === "content_block_delta" &&
    event.delta.type === "text_delta"
  ) {
    await publish(topic, JSON.stringify({ text: event.delta.text }), "token");
  }
}
```

**Local model (vLLM, llama.cpp, Ollama):** any of these expose an OpenAI-compatible streaming endpoint. Point the OpenAI client at it (`baseURL`) and the code above works unchanged.

**Bedrock, Vertex, etc.:** the streaming API has a different shape, but the structure (iterate, publish per delta) is the same.

## Performance notes for LLM token streaming over Mercure

- **Preserve order.** Await publications in sequence. Parallel HTTP requests can arrive out of order, including a `done` event arriving before the final chunk.
- **Batch small chunks.** Accumulate a short interval of text before publishing if per-chunk request overhead or plan limits are significant.
- **Handle failures.** Persist output and report a terminal failure through your application. Add sequence numbers if clients need to detect gaps or deduplicate retries.

## Authorization sketch

This payload excerpt grants access to one response. Add the required access-token claims and use a short expiry as described in [Authorization](../concepts/authorization.md).

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
        { "match": "https://example.com/conversations/42/responses/1" }
      ],
      "payload": { "user": "https://example.com/users/42" }
    }
  ]
}
```

Set it as the `__Secure-mercure_access_token` cookie with `Domain=example.com; Path=/.well-known/mercure; Secure; HttpOnly; SameSite=Strict`. The browser's `EventSource` picks it up automatically. See [Authorization](../concepts/authorization.md).

## Limits to be aware of

- **One `EventSource` per browser tab is enough.** Use multiple `match*` parameters before opening the stream. This hub accepts at most 100 matchers per subscription.
- **Connection counts.** A streaming chat keeps a connection open for the life of the page. The open-source hub has [no built-in cap](../concepts/reconnection-and-history.md#the-mercure-history-buffer): sizing is whatever your hardware can handle. [Mercure Cloud plans](https://mercure.rocks/pricing) provide managed capacity; [Enterprise](../production/high-availability.md) lets you scale across your own hubs.
- **Buffer size.** If you want a user reloading mid-stream to recover the in-progress answer, retain enough events for the expected recovery window across all active responses. Persist completed output in your database and refetch it if replay is incomplete.

## A complete Mercure LLM streaming reference

See [Awesome Mercure](../ecosystem/awesome.md) for integrations. The worker above still needs application routes for authorizing users, creating responses, and scheduling generation.

## Take your AI streaming to production

**[Mercure Cloud](https://mercure.rocks/pricing) handles the subscriber connections while you build the AI experience.** For deployments that need to keep prompts and responses on your own infrastructure, choose [Mercure Enterprise](../production/high-availability.md).

## Next steps

- [AI agent progress](ai-agent-progress.md): when there's more than tokens to stream.
- [Authorization](../concepts/authorization.md): minting per-conversation tokens.
- [Reconnection and history](../concepts/reconnection-and-history.md): surviving a mid-stream disconnect.
