---
title: "Async jobs and background progress over Mercure"
description: "Notify users when long-running background jobs progress or complete by publishing private updates to per-user Mercure topics."
---

# Mercure async jobs and progress

Start long-running jobs through an HTTP request, then publish progress and results from a worker. Mercure delivers those events to the requester without holding the initiating request open.

## Async job flow with Mercure

```text
   browser                origin                worker           hub
      |                     |                      |              |
      | POST /reports       |                      |              |
      | ------------------->| enqueue              |              |
      |   202 Accepted      | -------------------->|              |
      | <-------------------|                      |              |
      |   { jobId: "..." }  |                      |              |
      |                     |                      |              |
      | GET hub?match=...  |                      |              |
      | ----------------------------------------------------------|
      |                                            |              |
      |                                            | 100 rows     |
      |                                            | ------------>|
      | <---------------------------------------------------------|
      |                                            | 300 rows     |
      |                                            | ------------>|
      | <---------------------------------------------------------|
      |                                            | done + URL   |
      |                                            | ------------>|
      | <---------------------------------------------------------|
```

The browser holds an `EventSource` open from the moment the job is created until it completes. The origin server returns immediately and goes back to handling other requests.

## Originating an async job from the browser

```javascript
const res = await fetch("/api/reports", {
  method: "POST",
  body: JSON.stringify({ filters }),
  headers: { "Content-Type": "application/json" },
});
if (!res.ok) throw new Error(`Job creation failed: ${res.status}`);
const { jobId, userId } = await res.json();

const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append(
  "match",
  `https://example.com/users/${userId}/jobs/${jobId}`,
);

url.searchParams.set("last_event_id", "earliest");
const es = new EventSource(url, { withCredentials: true });
es.onmessage = (e) => {
  const update = JSON.parse(e.data);
  switch (update.type) {
    case "progress":
      progress.textContent = `${update.rows} rows processed`;
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

`last_event_id=earliest` covers a fast worker finishing before the browser subscribes. Because each job has a unique topic, it replays only that job's retained events, then receives live progress. Use a history-enabled transport.

The following snippets are pseudocode: `queue`, `JsonResponse`, database functions, and `publish` belong to your application. The `publish(topic, data, private=True)` helper must JSON-encode the data, send `private=on`, authenticate the request, and check the response status.

```python
def create_report(request):
    job_id = str(uuid.uuid4())
    queue.enqueue("generate_report", job_id, request.user.id, filters=request.json["filters"])
    return JsonResponse({"jobId": job_id, "userId": request.user.id}, status=202)
```

## Worker-side Mercure publishing

```python
def generate_report(job_id: str, user_id: str, filters: dict):
    topic = f"https://example.com/users/{user_id}/jobs/{job_id}"

    publish(topic, {"type": "started"}, private=True)

    rows = []
    for batch in query_batches(filters):
        rows.extend(batch)
        publish(
            topic, {"type": "progress", "rows": len(rows)},
            private=True,
        )

    url = save_report(rows)
    publish(topic, {"type": "done", "url": url}, private=True)
```

Each update goes to one per-user topic that embeds the owning user's ID. The user's access token authorizes a `subscribe` grant for `https://example.com/users/<their-id>/jobs/:id` (a `urlpattern` scoped to their ID), so they receive their own jobs but not anyone else's, even if they guess a `jobId`. See the [per-user authorization pattern](../concepts/authorization.md#per-user-authorization-on-shared-resources).

## When the user closes the tab

Closing a tab closes its SSE connection, but the worker can continue. On reopening, fetch the job status from your application and resume from a stored event cursor, or request `earliest` to replay retained events on the job topic.

For this to work end-to-end:

- The hub's history buffer must hold long enough to cover the longest expected job. With the open-source build and BoltDB, history is bounded by disk size (a generous default). For managed history limits, see the [current Cloud plans](https://mercure.rocks/pricing).
- Persist the job ID and status in your application. For multi-job streams, include `jobId` in every event payload.

## Reconnecting EventSource across client-side navigation

If your app uses client-side routing, keep the `EventSource` alive across route changes by hoisting it out of the component that started the job. A typical React shape:

```javascript
// JobsContext maintains a single EventSource that watches all of the user's in-flight jobs
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append(
  "match_urlpattern",
  `https://example.com/users/${userId}/jobs/:id`,
);
const es = new EventSource(url, { withCredentials: true });

es.onmessage = (e) => {
  // dispatch to whichever component cares about this jobId
};
```

This shared stream watches new events. On a full page load, fetch the current job statuses from your application and use a saved cursor if you need replay.

The page where the user originally clicked "Run" may unmount when they navigate away. The connection in the context provider doesn't.

## Reporting async job errors over Mercure

Workers fail. Make `failed` an event type and put the error message in `data`:

```python
try:
    generate(...)
except Exception:
    publish(topic, {"type": "failed", "error": "Report generation failed"}, private=True)
    raise
```

Persist terminal job status so the UI can recover even if the worker cannot publish an error. Let the queue handle retries, and distinguish a retryable attempt failure from a terminal job failure.

## Public job dashboards on Mercure

For an organization-only dashboard, keep `private=on` and grant access to the organization's members. Omit `private` only when the data is public; a team-shaped URL does not enforce authorization.

## When polling beats Mercure

Polling can be simpler when updates are infrequent and delayed progress is acceptable. Choose based on expected duration, update rate, and whether your application already uses Mercure; there is no universal time threshold.

## Next steps

- [LLM token streaming](llm-token-streaming.md): the same pattern with token-rate updates.
- [Authorization](../concepts/authorization.md): per-user job gating.
- [Reconnection and history](../concepts/reconnection-and-history.md): recovering from a closed tab.
