---
title: "In-app notifications and badge counters with Mercure"
description: "Deliver per-user notifications, badge counters, and broadcast announcements over Mercure with multi-tab consistency."
---

# Notifications

In-app notifications: mention badges, mailbox counters, "X started following you" toasts. The unsexy, ubiquitous case for real-time. Mercure handles it without a dedicated stack.

## What "notification" means here

Two flavors, with different topology:

| Flavor    | Example                     | Best fit                                  |
| --------- | --------------------------- | ----------------------------------------- |
| Per-user  | "You have 3 new messages"   | One topic per user, JWT-authorized.       |
| Broadcast | "System maintenance at 8pm" | A shared topic, no auth needed if public. |

You can ship both over the same connection.

## Per-user Mercure notifications

Each user subscribes to a topic that's theirs:

```javascript
// Per-User Mercure Notifications
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append(
  "match",
  `https://example.com/users/${userId}/notifications`,
);

const es = new EventSource(url, { withCredentials: true });
es.onmessage = (e) => {
  const notif = JSON.parse(e.data);
  showToast(notif);
  incrementBadge();
};
```

The cookie carries a JWT scoped to that user only:

```jsonc
// Per-User Mercure Notifications
{
  "mercure": {
    "subscribe": [
      { "match": "https://example.com/users/42/notifications" },
      { "match": "https://example.com/site/announcements" },
    ],
  },
  "exp": 1730000000,
}
```

The publisher (a comment service, a follow service, an order pipeline) emits the notification as a `private` update:

```python
# Per-User Mercure Notifications
def notify_user(user_id: str, payload: dict) -> None:
    publish(
        topic=f"https://example.com/users/{user_id}/notifications",
        data=json.dumps(payload),
        private=True,
    )
```

Because the update is `private` and the user's claim is the only one matching the topic, no one else receives it.

## Broadcast announcements over Mercure

Same connection, additional matcher:

```javascript
// Broadcast Announcements over Mercure
url.searchParams.append("match", "https://example.com/site/announcements");
```

Publish without `private=on`. Every connected user gets it. No JWT needed for this one if the announcement is public.

## Notification badge counters with Mercure

Two patterns, depending on how authoritative you need the count:

**1. Server tells you the count.** The notification payload includes the new total:

```jsonc
// Notification Badge Counters with Mercure
{ "type": "mention", "from": "alice", "unread": 7 }
```

The badge just renders `notif.unread`. Simple and always correct, at the cost of every notification carrying a count. Fine when you have one canonical "unread" definition.

**2. Client increments locally.** The payload is just the notification; the client adds 1 to its local count. The page resets the count on a separate event when the user reads it:

```javascript
// Notification Badge Counters with Mercure
es.addEventListener("read", (e) => {
  const { count } = JSON.parse(e.data);
  setBadge(count);
});
```

Lighter on each message but races with multi-tab usage. Mitigate by listening to `read` events the user generated in _another_ tab; the same SSE event reaches both tabs and they stay in sync.

## Multi-tab notification consistency

A user with three tabs open shouldn't get the same toast three times, but they should all see the badge update when one tab reads a message.

The shape that works:

- Show toasts in the most-recently-active tab only. Track activity via the `Page Visibility API` and the `BroadcastChannel` API; the active tab handles toasts, others suppress them.
- Update the badge in **every** tab. They all subscribe to the same topic and receive the same events.

This is a UI concern, not a Mercure concern. The hub delivers the same event to every connection; you decide what the UI does with it.

## Combining Mercure with Web Push for offline users

Mercure delivers to _connected_ clients. For a user with the app closed, you need [Web Push](https://web.dev/articles/push-notifications-overview) (or APNs / FCM on mobile). The two complement each other:

- User online -> Mercure pushes the in-app notification.
- User offline -> Web Push pings the OS notification center.

In your notify-user function, check connection state and dispatch to one or the other (or both). The [Active subscriptions API](../concepts/active-subscriptions.md#subscription-api) tells you whether the user is currently connected.

## Notification read receipts over Mercure

When the user opens a notification, post a `read` event to your origin, which publishes back over Mercure to update _all_ of the user's tabs:

```python
# Notification Read Receipts over Mercure
def mark_read(user_id: str, notif_id: str) -> None:
    db.mark_read(user_id, notif_id)
    publish(
        topic=f"https://example.com/users/{user_id}/notifications/read",
        data=json.dumps({"notif_id": notif_id, "count": db.unread_count(user_id)}),
        private=True,
    )
```

Each tab listens on `https://example.com/users/<id>/notifications/read` and updates its badge accordingly.

## Rate limiting publishers

A bug or a runaway loop that publishes a notification per millisecond is a real risk. Mitigations:

- **Coalesce on the publisher side**: debounce per user before emitting.
- **Hub-level rate limits.** The Cloud and Self-Hosted hubs can rate-limit publishers. The open-source hub can be put behind [Caddy's `ratelimit` module](https://github.com/mholt/caddy-ratelimit), which is included in the Mercure binary.

## Privacy and authorization

Notifications often carry personal data. A few rules:

- Always mark notification updates `private=on`.
- Authorize per-user: never use a wildcard subscriber matcher for notifications.
- Don't leak the topic's path in URLs that could end up in logs (avoid the `authorization` query parameter; use cookies).
- Consider [end-to-end encryption](../concepts/encryption.md) if the hub operator should not see the content.

## Next steps for Mercure notifications

- [Authorization](../concepts/authorization.md): minting per-user tokens.
- [Active subscriptions](../concepts/active-subscriptions.md): knowing whether the user is online.
- [Live data](live-data.md): for system-wide signals that aren't user-scoped.
