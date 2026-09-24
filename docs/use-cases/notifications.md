---
title: "In-app notifications and badge counters with Mercure"
description: "Deliver per-user notifications, badge counters, and broadcast announcements over Mercure with multi-tab consistency."
---

# Mercure notifications

Use Mercure for in-app notifications such as mentions, unread counts, and account activity. Publish private updates on a topic scoped to each user.

## What "notification" means here

Use a user topic or a shared announcement topic:

| Audience  | Example                     | Best fit                                  |
| --------- | --------------------------- | ----------------------------------------- |
| Per-user  | "You have 3 new messages"   | One topic per user, JWT-authorized.       |
| Broadcast | "System maintenance at 8pm" | A shared topic, no auth needed if public. |

You can ship both over the same connection.

## Per-user Mercure notifications

Each user subscribes to a topic that's theirs:

```javascript
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append(
  "match",
  `https://example.com/users/${userId}/notifications`,
);

const es = new EventSource(url, { withCredentials: true });
es.onmessage = (e) => {
  const notif = JSON.parse(e.data);
  showToast(notif);
  setBadge(notif.unread);
};
```

The cookie carries an access token scoped to that user only:

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
        { "match": "https://example.com/users/42/notifications" },
        { "match": "https://example.com/site/announcements" }
      ]
    }
  ]
}
```

The following pseudocode assumes an application `publish` helper that sends authenticated form-encoded requests. Include the current unread count in the payload and mark the update private:

```python
def notify_user(user_id: str, payload: dict) -> None:
    publish(
        topic=f"https://example.com/users/{user_id}/notifications",
        data=json.dumps(payload),
        private=True,
    )
```

The hub delivers the private update only to matching subscribers whose tokens grant access to that user's topic.

## Broadcast announcements over Mercure

Add this matcher before creating the `EventSource`; changing the URL object does not update an existing connection:

```javascript
url.searchParams.append("match", "https://example.com/site/announcements");
```

Publish without `private=on`. Every matching subscriber gets it. Tokenless connections require `anonymous` on the hub.

## Notification badge counters with Mercure

Two patterns, depending on how authoritative you need the count:

**1. Server tells you the count.** The notification payload includes the new total:

```json
{ "type": "mention", "from": "alice", "unread": 7 }
```

Render `notif.unread` as the authoritative count. Include a version or timestamp if concurrent publishers can produce counts out of order.

**2. Client increments locally.** The payload is just the notification; the client adds 1 to its local count. The page resets the count on a separate event when the user reads it:

```javascript
es.addEventListener("read", (e) => {
  const { count } = JSON.parse(e.data);
  setBadge(count);
});
```

Local increments need deduplication and periodic reconciliation with the server. Replayed notifications and concurrent tabs can otherwise inflate the count.

## Multi-tab notification consistency

A user with three tabs open shouldn't get the same toast three times, but they should all see the badge update when one tab reads a message.

Coordinate tab behavior:

- Show toasts in the most-recently-active tab only. Track activity via the `Page Visibility API` and the `BroadcastChannel` API; the active tab handles toasts, others suppress them.
- Update the badge in **every** tab. They all subscribe to the same topic and receive the same events.

This is a UI concern, not a Mercure concern. The hub delivers the same event to every connection; you decide what the UI does with it.

## Combining Mercure with Web Push for offline users

Mercure delivers to _connected_ clients. For a user with the app closed, you need [Web Push](https://web.dev/articles/push-notifications-overview) (or APNs / FCM on mobile). The two complement each other:

- User online -> Mercure pushes the in-app notification.
- User offline -> Web Push pings the OS notification center.

Presence can help choose a delivery channel, but a connected client is not proof that the user saw a notification. Persist unread state and use application acknowledgments when delivery matters.

## Notification read receipts over Mercure

When the user opens a notification, post a `read` event to your origin, which publishes back over Mercure to update _all_ of the user's tabs:

```python
def mark_read(user_id: str, notif_id: str) -> None:
    db.mark_read(user_id, notif_id)
    publish(
        topic=f"https://example.com/users/{user_id}/notifications",
        data=json.dumps({"notif_id": notif_id, "count": db.unread_count(user_id)}),
        type="read",
        private=True,
    )
```

Each tab uses `addEventListener("read", ...)` on its notifications stream, as shown above. The application's `publish` helper must pass `type="read"` as the Mercure form field.

## Rate limiting publishers

A bug or a runaway loop that publishes a notification per millisecond is a real risk. Mitigations:

- **Coalesce on the publisher side**: debounce per user before emitting.
- **Hub-level rate limits.** [Mercure Enterprise](https://mercure.rocks/pricing) includes [`caddy-ratelimit`](https://github.com/mholt/caddy-ratelimit), so you can configure publisher rate limits without building a custom binary. [Mercure Cloud](https://mercure.rocks/pricing) applies the limits of your plan. Adding this module to the open-source hub requires a custom build.

## Privacy and authorization

Notifications often carry personal data. A few rules:

- Always mark notification updates `private=on`.
- Authorize per-user: scope grants to that user's notification topics.
- Use opaque user identifiers in topics if logs must not contain personal information. Subscription topic names appear in request URLs; access tokens belong in cookies or headers.
- Consider [end-to-end encryption](../concepts/encryption.md) if the hub operator should not see the content.

## Next steps

- [Authorization](../concepts/authorization.md): minting per-user tokens.
- [Active subscriptions](../concepts/active-subscriptions.md): knowing whether the user is online.
- [Live data](live-data.md): for system-wide signals that aren't user-scoped.
