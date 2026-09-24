---
title: "Laravel Broadcasting with Mercure"
description: "Use Laravel's native Mercure broadcasting driver and Laravel Echo for real-time events. Choose managed Mercure Cloud or an Enterprise deployment on your own servers."
---

# Laravel Broadcasting with Mercure

[Laravel Broadcasting](https://laravel.com/docs/broadcasting) publishes application events to clients. [Laravel Echo](https://laravel.com/docs/broadcasting#client-side-installation) receives them in the browser. This guide uses Laravel's built-in Mercure driver.

**Keep Laravel's broadcasting API and choose where your real-time service runs.** [Mercure Cloud](https://mercure.rocks/pricing) is the managed option; [Mercure Enterprise](../production/high-availability.md) runs on your infrastructure. Both work with Laravel's native Mercure driver and Echo.

## Setting up

In a Laravel release that supports the Mercure installer, run:

```bash
php artisan install:broadcasting --mercure
```

## Configuring the hub

Configure Laravel's `MERCURE_URL`, `MERCURE_PUBLIC_URL`, and `MERCURE_JWT_SECRET`. The hub must trust the same signing secret and issuer. In the configuration below, tokens use `https://app.example.com` as `iss` and the public hub URL as `aud`. Match these to your Laravel `claims` configuration (the issuer defaults to `APP_URL`).

```caddyfile
mercure {
    issuer https://app.example.com {
        publisher {
            jwt {env.MERCURE_JWT_SECRET}
        }
        subscriber {
            jwt {env.MERCURE_JWT_SECRET}
        }
    }
    resource_identifier https://hub.example.com/.well-known/mercure

    # Publish presence events
    subscriptions

    # Required for whispers
    publish_origins https://app.example.com

    # Only if your hub and your app are not on the same origin
    cors_origins https://app.example.com
}
```

## Our first broadcast event

Create an event implementing `Illuminate\Contracts\Broadcasting\ShouldBroadcast`. Run a queue worker for queued broadcasts.

```php
<?php

// app/Events/MessageSent.php

namespace App\Events;

use Illuminate\Broadcasting\Channel;
use Illuminate\Broadcasting\InteractsWithSockets;
use Illuminate\Contracts\Broadcasting\ShouldBroadcast;
use Illuminate\Foundation\Events\Dispatchable;

class MessageSent implements ShouldBroadcast
{
    use Dispatchable, InteractsWithSockets;

    public function __construct(
        public string $content,
        public string $author,
    ) {}

    public function broadcastOn(): Channel
    {
        return new Channel('chat');
    }

    public function broadcastAs(): string
    {
        return 'message.sent';
    }

    /**
     * @return array<string, string>
     */
    public function broadcastWith(): array
    {
        return [
            'content' => $this->content,
            'author' => $this->author,
        ];
    }
}
```

## Send the event from a controller

Dispatch the event from an authenticated route with the `broadcast()` helper:

```php
<?php

// app/Http/Controllers/MessageController.php

namespace App\Http\Controllers;

use App\Events\MessageSent;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class MessageController extends Controller
{
    public function store(Request $request): JsonResponse
    {
        $validated = $request->validate([
            'content' => ['required', 'string', 'max:500'],
        ]);

        broadcast(new MessageSent($validated['content'], $request->user()->name))
            ->toOthers();

        return response()->json(['status' => 'sent']);
    }
}
```

`toOthers()` excludes the originating Echo connection when the request includes its `X-Socket-ID`. Other tabs belonging to the same user can still receive the event.

## Private channel

Import `Illuminate\Broadcasting\PrivateChannel` in the event class and add a `roomId` property. Replace `broadcastOn()` with the method below. Authorize members in `routes/channels.php`.

```php
public function broadcastOn(): Channel
{
    return new PrivateChannel('chat.' . $this->roomId); // Becomes "private-chat.1" on the hub
}
```

```php
<?php

// routes/channels.php

use App\Models\User;
use Illuminate\Support\Facades\Broadcast;

Broadcast::channel('chat.{roomId}', function (User $user, int $roomId) {
    return $user->rooms()->whereKey($roomId)->exists(); // true = authorized
});
```

With Mercure, each private update is published as a private update on the hub. Only subscribers whose token contains this channel can receive it.

## Presence channel

Import `Illuminate\Broadcasting\PresenceChannel` and add a `groupId` property. The authorization callback returns member data for presence listeners.

```php
public function broadcastOn(): Channel
{
    return new PresenceChannel('group.' . $this->groupId); // Becomes "presence-group.1" on the hub
}
```

```php
// routes/channels.php
Broadcast::channel('group.{groupId}', function (User $user, int $groupId) {
    if (! $user->groups()->whereKey($groupId)->exists()) {
        return false;
    }

    return ['id' => $user->id, 'name' => $user->name]; // Data shared with the other members
});
```

## End-to-end encryption

An encrypted channel hides the event name, payload, and socket ID from the hub. Routing metadata remains visible. Configure Laravel's `MERCURE_ENCRYPTION_KEY` as described in the [broadcasting documentation](https://laravel.com/docs/broadcasting#mercure).

```php
public function broadcastOn(): Channel
{
    return new PrivateEncryptedChannel('chat.' . $this->roomId); // Becomes "private-encrypted-chat.1" on the hub
}
```

Import `Illuminate\Broadcasting\PrivateEncryptedChannel` and define `roomId` on the event. It uses the same authorization as `PrivateChannel`, so our `chat.{roomId}` callback works for both.

## Laravel Echo

[Laravel Echo](https://laravel.com/docs/broadcasting#client-side-installation) listens to the events sent by Laravel. The install command already configures it with the Mercure broadcaster.

```javascript
// resources/js/echo.js
import Echo from "laravel-echo";

window.Echo = new Echo({
  broadcaster: "mercure",
  host: import.meta.env.VITE_MERCURE_HUB_URL, // If empty, Echo uses /.well-known/mercure on the current domain
});
```

## Listen to a public channel

```javascript
window.Echo.channel("chat").listen(".message.sent", (event) => {
  // The dot is needed because we used broadcastAs()
  console.log(`${event.author}: ${event.content}`);
});
```

Echo requests a Mercure token even for public channels:

```http
POST /broadcasting/auth
{"channel_names":["chat"]}

HTTP/1.1 200 OK
Set-Cookie: __Secure-mercure_access_token=<token>; Path=/.well-known/mercure; Domain=example.com; Secure; HttpOnly; SameSite=Strict
{"channel_names":[{"name":"chat"}],"expires_in":300,"topic_prefix":"https://laravel.alt/echo/","client_events":true}
```

Then it subscribes to the hub, the channel is now a Mercure topic:

```http
GET https://example.com/.well-known/mercure?match=https%3A%2F%2Flaravel.alt%2Fecho%2Fchannel%2Fchat
```

And when our controller broadcasts the event, the hub sends:

```text
id: <event-id>
data: {"channels":["chat"],"event":"message.sent","payload":{"content":"Hello!","author":"Bob"}}
```

## Listen to a private channel

```javascript
window.Echo.private("chat.1")
  .listen(".message.sent", (event) => {
    console.log(`${event.author}: ${event.content}`);
  })
  .error((error) => {
    // Called if the user is not authorized, or if the connection fails
    console.error(error);
  });
```

If the user is not authorized, the auth request still succeeds for the other channels but this one is flagged as denied:

```json
{
  "channel_names": [{ "name": "private-chat.1", "denied": true }],
  "expires_in": 300,
  "topic_prefix": "https://laravel.alt/echo/",
  "client_events": true
}
```

## Next steps

- [Laravel Broadcasting](https://laravel.com/docs/broadcasting): everything else about broadcasting in Laravel.
- [Laravel Echo](https://laravel.com/docs/broadcasting#client-side-installation): the client library.
