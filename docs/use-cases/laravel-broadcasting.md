---
title: "Laravel Broadcasting with Mercure"
description: "Using the broadcasting feature of Laravel with Mercure"
---

# Laravel Broadcasting

[Laravel Broadcasting](https://laravel.com/docs/broadcasting) is a feature of Laravel that allows your application to push server-side events to the client in real time, the web client uses [Laravel Echo](https://laravel.com/framework/docs/broadcasting#client-side-installation) to receives the events. It supports several drivers such as Mercure, Reverb, Pusher or Ably.

## Setting up

To set up Laravel Broadcasting with Mercure you only have to execute this command and follow the instructions:

```bash
php artisan install:broadcasting --mercure
```

## Configuring the hub

Laravel signs every token with `MERCURE_JWT_SECRET`, so your hub must use the same secret.

```caddyfile
# Configuring the hub
mercure {
    publisher_jwt {env.MERCURE_JWT_SECRET}
    subscriber_jwt {env.MERCURE_JWT_SECRET}

    # Required for presence channels, the browser publishes directly to the hub
    subscriptions

    # Required for whispers
    publish_origins <url>

    # Only if your hub and your app are not on the same origin
    cors_origins <url>
}
```

## Our first broadcast event

To create a broadcast event we have to create a class that implements `Illuminate\Contracts\Broadcasting\ShouldBroadcast`.

```php
<?php

// app/Events/MessageSent.php

namespace App\Events;

// Imports

class MessageSent implements ShouldBroadcast
{
    use Dispatchable, InteractsWithSockets;

    public function __construct(
        public string $content,
        public string $author,
    ) {}

    public function broadcastOn(): Channel // Channel where the event will be published
    {
        return new Channel('chat'); // Public channel: every client can receive the update. For private updates use PrivateChannel or PresenceChannel
    }

    public function broadcastAs(): string // The name of our event
    {
        return 'message.sent';
    }

    /**
     * @return array<string, string>
     */
    public function broadcastWith(): array // The data sent to the client
    {
        return [
            'content' => $this->content,
            'author' => $this->author,
        ];
    }
}
```

## Send the event from a controller

After we created our first broadcast event we can send it by using the `broadcast()` helper.

```php
<?php

// app/Http/Controllers/MessageController.php

namespace App\Http\Controllers;

// Imports

class MessageController extends Controller
{
    public function store(Request $request): JsonResponse
    {
        $validated = $request->validate([
            'content' => ['required', 'string', 'max:500'],
        ]);

        broadcast(new MessageSent($validated['content'], $request->user()->name))
            ->toOthers(); // The user who sent the message won't receive it

        return response()->json(['status' => 'sent']);
    }
}
```

## Private channel

A private channel only sends updates to users authorized in `routes/channels.php`.

```php
// Private channel
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

A presence channel is a private channel that also knows who is connected. Instead of returning `true`, the callback returns the data of the user.

```php
// Presence channel
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

With an encrypted channel, the hub never sees the content of your updates. Laravel encrypts the event name, the payload and the socket ID and only authorized users, who have the channel key, can decrypt.

```php
// End-to-end encryption
public function broadcastOn(): Channel
{
    return new PrivateEncryptedChannel('chat.' . $this->roomId); // Becomes "private-encrypted-chat.1" on the hub
}
```

It uses the same authorization as `PrivateChannel`, so our `chat.{roomId}` callback works for both.

## Laravel Echo

[Laravel Echo](https://laravel.com/framework/docs/broadcasting#client-side-installation) listens to the events sent by Laravel. The install command already configures it with the Mercure broadcaster.

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
// Listen to a public channel
window.Echo.channel("chat").listen(".message.sent", (event) => {
  // The dot is needed because we used broadcastAs()
  console.log(`${event.author}: ${event.content}`);
});
```

Echo first authenticates its channels even public ones to get a Mercure token:

```http
POST /broadcasting/auth
{"channel_names":["chat"]}

HTTP/1.1 200 OK
Set-Cookie: __Secure-mercure_access_token=<token>
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
// Listen to a private channel
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

## Next steps for Laravel over Mercure

- [Laravel Broadcasting](https://laravel.com/docs/broadcasting): everything else about broadcasting in Laravel.
- [Laravel Echo](https://laravel.com/framework/docs/broadcasting#client-side-installation): the client library.
