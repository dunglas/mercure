---
title: "Async jobs and background progress over Mercure"
description: "Notify users when long-running background jobs progress or complete by publishing private updates to per-user Mercure topics."
---

# Async jobs and progress

A user kicks off something slow: generate a report, transcode a video, run an analysis. The HTTP request that triggered it doesn't (and shouldn't) wait for completion. Mercure delivers the result, and any progress events along the way, when they're ready.

## Async job flow with Mercure

```text
# Async Job Flow with Mercure
   browser                origin                worker           hub
      |                     |                      |              |
      | POST /reports       |                      |              |
      | ------------------->| enqueue              |              |
      |   202 Accepted      | -------------------->|              |
      | <-------------------|                      |              |
      |   { jobId: "..." }  |                      |              |
      |                     |                      |              |
      | GET /sub?match=...  |                      |              |
      | ----------------------------------------------------------|
      |                                            |              |
      |                                            | progress 25% |
      |                                            | ------------>|
      | <---------------------------------------------------------|
      |                                            | progress 75% |
      |                                            | ------------>|
      | <---------------------------------------------------------|
      |                                            | done + URL   |
      |                                            | ------------>|
      | <---------------------------------------------------------|
```

The browser holds an `EventSource` open from the moment the job is created until it completes. The origin server returns immediately and goes back to handling other requests.

## Originating an async job from the browser

```javascript
// Originating an Async Job from the Browser
const res = await fetch("/api/reports", {
  method: "POST",
  body: JSON.stringify({ filters }),
  headers: { "Content-Type": "application/json" },
});
const { jobId, userId } = await res.json();

const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append(
  "match",
  `https://example.com/users/${userId}/jobs/${jobId}`,
);

const es = new EventSource(url, { withCredentials: true });
es.onmessage = (e) => {
  const update = JSON.parse(e.data);
  switch (update.type) {
    case "progress":
      bar.value = update.percent;
      break;
    case "done":
      window.location = update.url;
      es.close();
      break;
    case "failed":
      showError(update.error);
      es.close();
      break;
  }
};
```

The origin server enqueues the job and returns the ID:

```python
# Originating an Async Job from the Browser
def create_report(request):
    job_id = str(uuid.uuid4())
    queue.enqueue("generate_report", job_id, request.user.id, filters=request.json["filters"])
    return JsonResponse({"jobId": job_id, "userId": request.user.id}, status=202)
```

## Worker-side Mercure publishing

```python
# Worker-Side Mercure Publishing
def generate_report(job_id: str, user_id: str, filters: dict):
    topic = f"https://example.com/users/{user_id}/jobs/{job_id}"

    publish(topic, {"type": "started"}, private=True)

    rows = []
    for i, batch in enumerate(query_batches(filters)):
        rows.extend(batch)
        publish(
            topic, {"type": "progress", "percent": i * 100 // batch_count},
            private=True,
        )

    url = save_report(rows)
    publish(topic, {"type": "done", "url": url}, private=True)
```

Each update goes to one per-user topic that embeds the owning user's id. The user's access token authorizes a `subscribe` grant for `https://example.com/users/<their-id>/jobs/:id` (a `URLPattern` scoped to their id), so they receive their own jobs but not anyone else's, even if they guess a `jobId`. See the [per-user authorization pattern](../concepts/authorization.md#per-user-authorization-on-shared-resources).

## When the user closes the tab

The browser-side `EventSource` is gone, but the worker keeps running and keeps publishing. The hub buffers updates in its history. When the user opens the page again (perhaps from a "your report is ready" email), the new `EventSource` includes `lastEventID` and the hub replays everything that happened. The user sees the final progress and the download link without polling.

For this to work end to end:

- The hub's history buffer must hold long enough to cover the longest expected job. With the open-source build and BoltDB, history is bounded by disk size (a generous default). Cloud tiers cap it at 100-5,000 messages depending on plan.
- The page that re-subscribes must know the `jobId`. Persist it (cookie, local DB) when you submit the job.

> **Pro tip.** For long-running batch jobs (hours), keep the history in Postgres or Kafka via [Self-Hosted Mercure](https://mercure.rocks/pricing). The Postgres transport doubles as a queryable event store: you can join job history with the rest of your data in SQL.

## Reconnecting EventSource across client-side navigation

If your app uses client-side routing, keep the `EventSource` alive across route changes by hoisting it out of the component that started the job. A typical React shape:

```javascript
// JobsContext maintains a single EventSource that watches all of the user's in-flight jobs
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append(
  "matchURLPattern",
  `https://example.com/users/${userId}/jobs/:id`,
);
const es = new EventSource(url, { withCredentials: true });

es.onmessage = (e) => {
  // dispatch to whichever component cares about this jobId
};
```

The page where the user originally clicked "Run" may unmount when they navigate away. The connection in the context provider doesn't.

## Reporting async job errors over Mercure

Workers fail. Make `failed` an event type and put the error message in `data`:

```python
# Reporting Async Job Errors over Mercure
try:
    generate(...)
except Exception as e:
    publish(topic, {"type": "failed", "error": str(e)}, private=True)
    raise
```

Don't bury failures. A worker that dies without publishing a terminal event leaves the UI hung. Catch broadly, publish, then re-raise so your queue's retry logic still kicks in.

## Public job dashboards on Mercure

If the goal is "anyone in the org can watch this job," publish to a shared room/team topic without `private=on`. Authorize by matching the room/team URL instead. Public job streams are a common pattern for CI dashboards and shared deploy boards.

## When polling beats Mercure

For jobs that are usually fast (under a few seconds), a short poll loop ("retry every second for 30 seconds") may be simpler than a Mercure subscription. The break-even is somewhere around 5-10 seconds of expected duration: above that, the SSE connection is cheaper than repeated HTTP requests; below that, the connection setup outweighs the savings.

## Next steps for async jobs with Mercure

- [LLM token streaming](llm-token-streaming.md): the same pattern with token-rate updates.
- [Authorization](../concepts/authorization.md): per-user job gating.
- [Reconnection and history](../concepts/reconnection-and-history.md): recovering from a closed tab.
