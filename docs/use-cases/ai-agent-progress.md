---
title: "Real-time AI agent progress and state sync with Mercure"
description: "Push tool calls, step transitions, and live state from a running AI agent to the browser using structured events on Mercure topics."
---

# Stream AI agent progress with Mercure

An agent may spend time searching, calling tools, and processing results. Publish structured progress events so the UI can show the current step and final output.

This guide pushes structured agent state to the UI in real time using Mercure.

## Streaming structured AI agent events over Mercure

For a token stream you push `text` chunks. For an agent you push **events** that describe what just happened:

```jsonl
{ "type": "step.started",      "step": "search_web",       "input": {"query": "mercure protocol"} }
{ "type": "tool.called",       "tool": "fetch",            "url":   "https://mercure.rocks" }
{ "type": "tool.completed",    "tool": "fetch",            "bytes": 14732 }
{ "type": "step.completed",    "step": "search_web",       "results": 5 }
{ "type": "step.started",      "step": "summarize" }
{ "type": "token",             "text": "Mercure is a..." }
{ "type": "token",             "text": " protocol for..." }
{ "type": "step.completed",    "step": "summarize" }
{ "type": "run.completed",     "output": "..." }
```

The browser updates its state from these events. The example below assumes sequential steps; concurrent steps need unique step IDs and separate state.

## Topics

Give each run a topic scoped to its owner. The browser and worker must use the same `userId` and `runId`.

```javascript
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append(
  "match",
  `https://example.com/users/${userId}/runs/${runId}`,
);

url.searchParams.set("last_event_id", "earliest");
const es = new EventSource(url, { withCredentials: true });
const state = { steps: [], output: "" };

es.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  switch (msg.type) {
    case "step.started":
      state.steps.push({ name: msg.step, status: "running" });
      break;
    case "step.completed":
      state.steps[state.steps.length - 1].status = "done";
      break;
    case "token":
      state.output += msg.text;
      break;
    case "run.completed":
      state.output = msg.output ?? state.output;
      es.close();
      break;
  }
  render(state);
};
```

To watch several runs, add a URL Pattern before creating the `EventSource`. Keep state per run and do not close the shared stream when one run completes:

```javascript
url.searchParams.append(
  "match_urlpattern",
  `https://example.com/users/${userId}/runs/:id`,
);
```

Include `runId` in each payload when multiplexing runs. SSE event IDs are replay cursors; they do not identify the topic or run unless your application explicitly encodes that information.

## Publisher: a Python agent

This Python worker sketch requires `openai` and `requests`, the `OPENAI_API_KEY`, `OPENAI_MODEL`, and `MERCURE_PUBLISHER_JWT` environment variables, and application-provided `TOOLS` and `TOOLS_IMPL`. Authenticate and authorize the run before scheduling it.

```python
import json
import os
import requests
from openai import OpenAI

HUB = "https://hub.example.com/.well-known/mercure"
PUBLISHER_JWT = os.environ["MERCURE_PUBLISHER_JWT"]

def publish(topic: str, event: dict, type_: str = "message") -> None:
    response = requests.post(
        HUB,
        headers={"Authorization": f"Bearer {PUBLISHER_JWT}"},
        data={"topic": topic, "data": json.dumps(event), "type": type_, "private": "on"},
        timeout=10,
    )
    response.raise_for_status()

def run_agent(user_id: str, run_id: str, prompt: str) -> None:
    topic = f"https://example.com/users/{user_id}/runs/{run_id}"
    publish(topic, {"type": "run.started", "prompt": prompt})

    client = OpenAI()
    messages = [{"role": "user", "content": prompt}]

    while True:
        publish(topic, {"type": "step.started", "step": "model"})
        response = client.chat.completions.create(
            model=os.environ["OPENAI_MODEL"],
            messages=messages,
            tools=TOOLS,
        )
        msg = response.choices[0].message
        publish(topic, {"type": "step.completed", "step": "model"})

        if not msg.tool_calls:
            publish(topic, {"type": "run.completed", "output": msg.content})
            return

        messages.append(msg.model_dump(exclude_none=True))
        for call in msg.tool_calls:
            publish(topic, {
                "type": "tool.called",
                "tool": call.function.name,
                "args": json.loads(call.function.arguments),
            })
            result = TOOLS_IMPL[call.function.name](**json.loads(call.function.arguments))
            publish(topic, {"type": "tool.completed", "tool": call.function.name})

            messages.append({
                "role": "tool",
                "tool_call_id": call.id,
                "content": json.dumps(result),
            })
```

Same pattern with [Anthropic's tool use](https://docs.anthropic.com/en/docs/build-with-claude/tool-use), Vercel AI SDK, LangGraph, or your own harness. The events you emit are yours to design.

## Per-user run topics

For private runs, scope the topic to the user that owns it:

```python
USER_TOPIC = f"https://example.com/users/{user_id}/runs/{run_id}"

publish(topic=USER_TOPIC, event=event)
```

Private delivery requires both `private=on` on the publication and a subscriber grant scoped to the owner's topic. Knowing a topic name does not grant access to its private updates.

This is the [per-user authorization pattern](../concepts/authorization.md#per-user-authorization-on-shared-resources) applied to agent runs.

## What the UI gets for free

With a history-enabled transport, reconnecting clients can replay retained progress events. A new tab must request history explicitly, as the example does with `last_event_id=earliest`.

Persist the run's status and output in your application so clients can recover after history expires. Add a `runId` to payloads when one stream covers several runs, and close the stream only when all watched runs finish.

## Cancel a run

Send a `POST` from the browser to a small origin endpoint that flips a flag the agent harness checks between steps. The harness publishes a `run.cancelled` event and exits. There's no direct "cancel this Mercure subscription". Mercure only carries the state, not the control plane.

## Backpressure for AI agent event streams

Agents can emit events faster than a UI can render them. Measure the publish rate and account for hub limits.

Two practical mitigations:

- **Coalesce on the publisher side.** Group rapid events of the same type before publishing.
- **Throttle on the subscriber side.** Use `requestAnimationFrame` to batch state updates instead of rendering on every message.

## Authorization sketch

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
          "match": "https://example.com/users/42/runs/:runId",
          "match_type": "urlpattern"
        }
      ],
      "payload": { "username": "alice" }
    }
  ]
}
```

The hub assigns each connection a random `urn:uuid:` subscriber ID; clients can't choose it. For a stable per-user identity across the user's tabs (convenient if you also want to surface presence, see [Active subscriptions](../concepts/active-subscriptions.md)), put it in the `subscribe` grant's `payload`. For instance, "Alice is watching this run" pills on a shared dashboard.

## When this is overkill

Return the result through the initiating HTTP request when the run is short and only one client needs it. Use Mercure when users need intermediate progress, when several clients watch a run, or when generation runs in a background worker.

## Next steps

- [LLM token streaming](llm-token-streaming.md): for the simpler "just stream tokens" case.
- [Active subscriptions](../concepts/active-subscriptions.md): show who else is watching the run.
- [Authorization](../concepts/authorization.md): per-user run gating.
