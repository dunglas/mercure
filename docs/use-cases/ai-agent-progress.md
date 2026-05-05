---
title: "Real-time AI agent progress and state sync with Mercure"
description: "Push tool calls, step transitions, and live state from a running AI agent to the browser using structured events on Mercure topics."
---

# AI agent progress

When an LLM is just responding, you stream tokens. When it's an _agent_ (calling tools, searching the web, reading files, branching into sub-tasks), there's a lot more state to communicate than text deltas. The user wants to know "what is it doing right now?" and "how far along is it?".

This guide pushes structured agent state to the UI in real time using Mercure.

## Streaming structured AI agent events over Mercure

For a token stream you push `text` chunks. For an agent you push **events** that describe what just happened:

```jsonc
// Streaming Structured AI Agent Events over Mercure
{ "type": "step.started",      "step": "search_web",       "input": {"query": "mercure protocol"} }
{ "type": "tool.called",       "tool": "fetch",            "url":   "https://mercure.rocks" }
{ "type": "tool.completed",    "tool": "fetch",            "bytes": 14732 }
{ "type": "step.completed",    "step": "search_web",       "results": 5 }
{ "type": "step.started",      "step": "summarize" }
{ "type": "token",             "text": "Mercure is a..." }
{ "type": "token",             "text": " protocol for..." }
{ "type": "step.completed",    "step": "summarize" }
{ "type": "run.completed",     "summary": "..." }
```

The browser keeps a state machine fed by these events: a status line ("searching the web..."), a step list, partial output. The UI can render whatever fidelity you want (collapsed status pill, full timeline, debug view) without the server needing to know which.

## Topics

A run gets its own topic. Subscribe to that topic and you receive everything happening in that run:

```javascript
// Topics
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", `https://example.com/runs/${runId}`);

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
      es.close();
      break;
  }
  render(state);
};
```

A user with several runs in flight (say, a chat with multiple turns or a dashboard of background agents) opens **one** `EventSource` and uses `matchURLPattern`:

```javascript
// Topics
url.searchParams.append("matchURLPattern", "https://example.com/runs/:id");
```

Now every run the user is allowed to see flows over the same connection. The `id` field on each event tells you which run it belongs to. Or set the topic per-event and read it from the SSE `id`.

## Publisher: a Python agent

A pseudocode harness for a tool-using agent that emits events as it goes:

```python
# Publisher: a Python agent
import json
import requests
from openai import OpenAI

HUB = "https://hub.example.com/.well-known/mercure"
PUBLISHER_JWT = os.environ["MERCURE_PUBLISHER_JWT"]

def publish(topic: str, event: dict, type_: str = "message") -> None:
    requests.post(
        HUB,
        headers={"Authorization": f"Bearer {PUBLISHER_JWT}"},
        data={"topic": topic, "data": json.dumps(event), "type": type_},
        timeout=2,
    )

def run_agent(run_id: str, prompt: str) -> None:
    topic = f"https://example.com/runs/{run_id}"
    publish(topic, {"type": "run.started", "prompt": prompt})

    client = OpenAI()
    messages = [{"role": "user", "content": prompt}]

    while True:
        publish(topic, {"type": "step.started", "step": "model"})
        response = client.chat.completions.create(
            model="gpt-4o",
            messages=messages,
            tools=TOOLS,
        )
        msg = response.choices[0].message
        publish(topic, {"type": "step.completed", "step": "model"})

        if not msg.tool_calls:
            publish(topic, {"type": "run.completed", "output": msg.content})
            return

        messages.append(msg)
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

## Two coordinates: run topic and user topic

For most apps you want both:

```python
# Two coordinates: run topic and user topic
RUN_TOPIC  = f"https://example.com/runs/{run_id}"
USER_TOPIC = f"https://example.com/users/{user_id}/runs/{run_id}"

publish(topic=RUN_TOPIC, alternate=USER_TOPIC, data=event, private=True)
```

The run topic identifies the work. The user topic gates access: only the user that owns the run is authorized for that alternate, so even if someone guesses the run ID they can't subscribe to it.

This is the [per-user authorization pattern](../concepts/authorization.md#per-user-authorization-on-shared-topics) applied to agent runs.

## What the UI gets for free

Because every event has a Mercure event ID and the hub buffers history:

- **Reconnect resilience.** User closes the laptop mid-run and opens it again; the UI reconnects and replays the events it missed. No dropped progress.
- **Late join.** A second tab opened halfway through a run sees the run from the start (if the buffer is sized for it). Useful for "share this run" links.
- **Cross-device.** A user starts a run on desktop, walks away, and the same run shows up on their phone if it's listening to the same user topic.

## Cancel a run

Send a `POST` from the browser to a small origin endpoint that flips a flag the agent harness checks between steps. The harness publishes a `run.cancelled` event and exits. There's no direct "cancel this Mercure subscription". Mercure only carries the state, not the control plane.

## Backpressure for AI agent event streams

Tool-heavy agents can produce a lot of events (an agent that runs hundreds of small tool calls in a loop will publish thousands of messages). The hub takes them all, but the UI may struggle to render them fast enough.

Two practical mitigations:

- **Coalesce on the publisher side.** Group rapid events of the same type before publishing.
- **Throttle on the subscriber side.** Use `requestAnimationFrame` to batch state updates instead of rendering on every message.

## Authorization sketch

```jsonc
// Authorization sketch
{
  "mercure": {
    "subscribe": [
      {
        "match": "https://example.com/users/42/runs/:runId",
        "matchType": "URLPattern",
      },
    ],
    "subscriber": "urn:uuid:user-42",
    "payload": { "username": "alice" },
  },
}
```

The `subscriber` field gives you a stable identity across the user's tabs, which is convenient if you also want to surface presence (see [Active subscriptions](../concepts/active-subscriptions.md)). For instance, "Alice is watching this run" pills on a shared dashboard.

## When this is overkill

If your agent finishes in a few seconds and the only thing you'd push is a final result, just `await` the call from the browser. Mercure earns its keep when:

- runs take long enough that users want to _see_ progress, not just wait;
- the same agent state needs to reach multiple clients (multi-tab, multi-device, observers);
- you're already running an agent worker and don't want to keep request-handling threads tied to it.

## Next steps for AI agent streaming with Mercure

- [LLM token streaming](llm-token-streaming.md): for the simpler "just stream tokens" case.
- [Active subscriptions](../concepts/active-subscriptions.md): show who else is watching the run.
- [Authorization](../concepts/authorization.md): per-user run gating.
