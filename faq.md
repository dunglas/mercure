## FAQ

### How to Use Mercure with GraphQL?

Because they are delivery agnostic, Mercure plays particulary well with [GraphQL's subscriptions](https://facebook.github.io/graphql/draft/#sec-Subscription).

In response to the subscription query, the GraphQL server may return a corresponding topic URL.
The client can then subscribe to the Mercure's event stream corresponding to this subscription by creating a new `EventSource` with an URL like `https://example.com/hub?topic=https://example.com/subscriptions/<subscription-id>` as parameter.

Updates for the given subscription can then be sent from the GraphQL server to the clients through the Mercure hub (in the `data` property of the server-sent event).

To unsubscribe, the client just calls `EventSource.close()`.

Mercure can easily be integrated with Apollo GraphQL by creating [a dedicated transport](https://github.com/apollographql/graphql-subscriptions).

### How to Monitor the Hub Using Supervisor

Use the following file as a template to run the Mercure hub with [Supervisor](http://supervisord.org):

```ini
[program:mercure]
command=/path/to/mercure
process_name=%(program_name)s_%(process_num)s
numprocs=1
environment=JWT_KEY="!ChangeMe!"
directory=/tmp
autostart=true
autorestart=true
startsecs=5
startretries=10
user=www-data
redirect_stderr=false
stdout_capture_maxbytes=1MB
stderr_capture_maxbytes=1MB
stdout_logfile=/path/to/logs/mercure.out.log
stderr_logfile=/path/to/logs/mercure.error.log
```

Save this file to `/etc/supervisor/conf.d/mercure.conf`.
Run `supervisorctl reread` and `supervisorctl update` to activate and start the Mercure hub.

### How to Use NGINX as an HTTP/2 Reverse Proxy in Front of the Hub?

[NGINX](https://www.nginx.com) is supported out of the box. Use the following proxy configuration:

```nginx
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;

    ssl_certificate /path/to/ssl/cert.crt;
    ssl_certificate_key /path/to/ssl/cert.key;

    location / {
        proxy_pass http://url-of-your-mercure-hub;
        proxy_read_timeout 24h;
        proxy_http_version 1.1;
        proxy_set_header Connection "";

        ## Be sure to set USE_FORWARDED_HEADERS=1 to allow the hub to use those headers ##
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### What's the Difference Between Mercure and WebSocket?

[WebSocket](https://developer.mozilla.org/en-US/docs/Web/API/WebSockets_API) is a low level protocol, Mercure is a high level one.
Mercure provides convenient built-in features such as authorization, re-connection and state reconciliation ; while with WebSocket, you need to implement them yourself.
Also, unlike Mercure (which is built on top of HTTP and Server-Sent Events), WebSocket [is not designed to leverage HTTP/2](https://www.infoq.com/articles/websocket-and-http2-coexist).

HTTP/2 connections are multiplexed and bidirectional by default (it was not the case of HTTP/1).
When using Mercure over a h2 connection (recommended), your app can receive data through Server-Sent Events, and send data to the server with regular `POST` (or `PUT`/`PATCH`/`DELETE`) requests, with no overhead.

Basically, in most cases Mercure can be used as a modern and easier to use replacement for WebSocket.

### What's the Difference Between Mercure and WebSub?

[WebSub](https://www.w3.org/TR/websub/) is a server-to-server only protocol, while Mercure is also a server-to-client and client-to-client protocol.

Mercure has been heavily inspired by WebSub, and we tried to make the protocol as close as possible from the WebSub one.

Mercure uses Server-Sent Events to dispatch the updates, while WebSub use `POST` requests. Also, Mercure has an advanced authorization mechanism, and allows to subscribe to several topics with only one connection using URI templates.

### What's the Difference Between Mercure and Web Push?

The [Push API](https://developer.mozilla.org/en-US/docs/Web/API/Push_API) is a simplex protocol [mainly designed](https://developers.google.com/web/fundamentals/push-notifications/) to send [notifications](https://developer.mozilla.org/en-US/docs/Web/API/Notifications_API) to devices currently not connected to the application.
In most implementations, the size of the payload to dispatch is very limited, and the messages are sent through the proprietary APIs and servers of the browsers' and operating systems' vendors.

On the other hand, Mercure is a duplex protocol designed to send live updates to devices currently connected to the web or mobile app. The payload is not limited, and the message goes directly from your servers to the clients.

In summary, use the Push API to send notifications to offline users (that will be available in Chrome, Android and iOS's notification centers), and use Mercure to receive and publish live updates when the user is using the app.