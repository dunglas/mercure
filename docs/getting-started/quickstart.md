---
title: "Mercure quickstart: subscribe and publish in 5 minutes"
description: "Run the Mercure.rocks Hub locally with Docker, subscribe with EventSource, and publish your first real-time update with curl."
---

# Quickstart

This guide gets you from zero to a real-time update in your browser in five minutes. We'll run a hub locally with Docker, subscribe from a one-liner HTML page, and publish from `curl`.

If you already have a hub running, jump to [Subscribe](#subscribe-to-a-mercure-topic-from-the-browser) or [Publish](#publish-a-mercure-update-with-curl).

## Run the Mercure hub locally with Docker

```console
# Run the Mercure Hub Locally with Docker
docker run -p 8080:80 \
  -e SERVER_NAME=':80' \
  -e MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
  -e MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
  -e MERCURE_RESOURCE_IDENTIFIER='https://localhost/.well-known/mercure' \
  -e MERCURE_EXTRA_DIRECTIVES='anonymous
cors_origins *
demo' \
  dunglas/mercure
```

The hub is now serving on `http://localhost:8080`.

What that command does:

- `MERCURE_*_JWT_KEY`: the secret used to verify access tokens. Don't ship this value to production; the [installation guide](installation.md) covers proper key management.
- `MERCURE_RESOURCE_IDENTIFIER`: the OAuth 2.0 audience tokens must carry in their `aud` claim. The demo token below uses this exact value.
- `anonymous`: lets clients subscribe to public topics without a token (handy in dev, off by default in prod).
- `cors_origins *`: allow any origin to connect (you'll want to restrict this).
- `demo`: turns on the in-browser debugger at <http://localhost:8080/.well-known/mercure/ui/>.

> **Pro tip.** Don't want to manage a hub? [Mercure Cloud](https://mercure.rocks/pricing) has a free tier sized for prototyping. Same protocol, no infrastructure to run.

## Subscribe to a Mercure topic from the browser

Save this as `index.html` and open it in your browser:

```html
<!-- index.html -->
<!doctype html>
<title>Mercure quickstart</title>
<ul id="log"></ul>
<script>
  const url = new URL("http://localhost:8080/.well-known/mercure");
  url.searchParams.append("match", "https://example.com/books/1");

  const es = new EventSource(url);
  es.onmessage = (event) => {
    const li = document.createElement("li");
    li.textContent = event.data;
    document.getElementById("log").prepend(li);
  };
</script>
```

The `match` query parameter does an exact-match subscription on the topic `https://example.com/books/1`. To subscribe to a family of URLs at once, use `match_urlpattern`:

```javascript
// Subscribe to a Mercure Topic from the Browser
url.searchParams.append("match_urlpattern", "https://example.com/books/:id");
```

URL patterns follow the [WHATWG URL Pattern](https://urlpattern.spec.whatwg.org) syntax. They replace URI templates as the recommended templating language for URL topics. [Topics and matchers](../concepts/topics-and-matchers.md) covers the full set.

## Publish a Mercure update with curl

In another terminal:

```console
# Publish a Mercure Update with curl
curl -X POST http://localhost:8080/.well-known/mercure \
  -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6ImF0K2p3dCJ9.eyJhdWQiOiJodHRwczovL2xvY2FsaG9zdC8ud2VsbC1rbm93bi9tZXJjdXJlIiwiYXV0aG9yaXphdGlvbl9kZXRhaWxzIjpbeyJhY3Rpb25zIjpbInB1Ymxpc2giXSwidG9waWNzIjpbeyJtYXRjaCI6IioifV0sInR5cGUiOiJtZXJjdXJlIn0seyJhY3Rpb25zIjpbInN1YnNjcmliZSJdLCJ0b3BpY3MiOlt7Im1hdGNoIjoiKiJ9XSwidHlwZSI6Im1lcmN1cmUifV0sImV4cCI6NDEwMjQ0NDgwMH0.AIQorI3mpd1OEJXqqgnnuTid7QDJgQFfkv8pNb4cDR0' \
  -d 'topic=https://example.com/books/1' \
  -d 'data={"status": "checked out"}'
```

Reload the browser tab. The new message appears at the top of the list.

The bearer token is an OAuth 2.0 access token signed with the dev key above (header `{ "alg": "HS256", "typ": "at+jwt" }`), carrying:

```jsonc
// Publish a Mercure Update with curl
{
  "aud": "https://localhost/.well-known/mercure",
  "exp": 4102444800,
  "authorization_details": [
    { "type": "mercure", "actions": ["publish"], "topics": [{ "match": "*" }] },
    {
      "type": "mercure",
      "actions": ["subscribe"],
      "topics": [{ "match": "*" }],
    },
  ],
}
```

The `aud` matches the hub's `resource_identifier`, the `typ` header is `at+jwt`, and the `publish` grant covers every topic. Generate your own at [jwt.io](https://jwt.io). Details in [Authorization](../concepts/authorization.md).

## Closing the Mercure EventSource connection

`EventSource` keeps the TCP connection open as long as the page lives. Single-page apps in particular should call `es.close()` when the component that opened the stream unmounts:

```javascript
// Closing the Mercure EventSource Connection
useEffect(() => {
  const es = new EventSource(url);
  es.onmessage = (e) => /* ... */;
  return () => es.close();
}, [url]);
```

Otherwise, the browser keeps the connection alive on cached pages and the hub keeps the slot allocated.

## Mercure quickstart: publish/subscribe flow recap

```text
# Mercure Quickstart: Publish/Subscribe Flow Recap
            POST /.well-known/mercure       GET /.well-known/mercure?match=...
publisher  ----------------------->  hub  <-----------------------------  subscriber
                                  (HTTP/2,                              (Server-Sent
                                   one TCP                               Events,
                                   per client)                           one TCP)
```

The hub is the only piece you need to deploy. Publishers can be anywhere: your existing API server, a worker, a serverless function, a GitHub webhook. Subscribers use plain `EventSource`, so anything that talks HTTP can subscribe.

## Mercure quickstart next steps

- **Learn the protocol surface**: [Topics and matchers](../concepts/topics-and-matchers.md), [Authorization](../concepts/authorization.md).
- **Build something concrete**: the [LLM streaming](../use-cases/llm-token-streaming.md) and [AI agent progress](../use-cases/ai-agent-progress.md) guides each ship a working example.
- **Move toward production**: [Configuration](../deployment/configuration.md), [Health checks](../production/health-monitoring.md), [Rolling updates](../production/rolling-updates.md).
