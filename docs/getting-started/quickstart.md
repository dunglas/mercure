---
title: "Mercure quickstart: subscribe and publish in 5 minutes"
description: "Run the Mercure.rocks Hub locally with Docker, subscribe with EventSource, and publish your first real-time update with curl."
---

# Mercure quickstart

Run a local Mercure hub with Docker, subscribe from a browser, and publish an update with `curl`. You need Docker, a browser, and `curl`.

If you already have a hub running, jump to [Subscribe](#subscribe-to-a-mercure-topic-from-the-browser) or [Publish](#publish-a-mercure-update-with-curl).

## Run the Mercure hub locally with Docker

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp \
  -e MERCURE_EXTRA_DIRECTIVES=playground \
  dunglas/mercure
```

The hub is now serving on `https://localhost`.

What that command does:

- `-p 443:443/udp`: Caddy serves HTTP/3 over QUIC on this port too. Without it, the container still starts and HTTP/1.1 and HTTP/2 both work, but clients silently fall back past HTTP/3.
- `MERCURE_EXTRA_DIRECTIVES=playground`: turns on the insecure playground, which enables anonymous subscriptions, permissive CORS, and the in-browser debugger at <https://localhost/.well-known/mercure/debug/> with a prefilled all-access token signed with a well-known default secret (no `MERCURE_*_JWT_KEY` needed). Drop it for production; the [installation guide](installation.md) covers the default config and proper key management.

The default `SERVER_NAME` is `localhost`. Caddy uses its local certificate authority for HTTPS. Open <https://localhost/.well-known/mercure/debug/> and trust the local certificate before subscribing. The development token below uses the matching issuer and audience.

## Subscribe to a Mercure topic from the browser

Save this as `index.html` and open it in your browser:

```html
<!-- index.html -->
<!doctype html>
<title>Mercure quickstart</title>
<ul id="log"></ul>
<script>
  const url = new URL("https://localhost/.well-known/mercure");
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
const allBooks = new URL("https://localhost/.well-known/mercure");
allBooks.searchParams.append(
  "match_urlpattern",
  "https://example.com/books/:id",
);
const books = new EventSource(allBooks);
books.onmessage = (event) => console.log(event.data);
```

URL patterns follow the [WHATWG URL Pattern](https://urlpattern.spec.whatwg.org) syntax. They replace URI templates as the recommended templating language for URL topics. [Topics and matchers](../concepts/topics-and-matchers.md) covers the full set.

## Publish a Mercure update with `curl`

In another terminal:

```console
curl --insecure -X POST https://localhost/.well-known/mercure \
  -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6ImF0K2p3dCJ9.eyJhdWQiOiJodHRwczovL2xvY2FsaG9zdC8ud2VsbC1rbm93bi9tZXJjdXJlIiwiYXV0aG9yaXphdGlvbl9kZXRhaWxzIjpbeyJhY3Rpb25zIjpbInB1Ymxpc2giXSwidG9waWNzIjpbeyJtYXRjaCI6IioifV0sInR5cGUiOiJodHRwczovL21lcmN1cmUucm9ja3MvYXV0aG9yaXphdGlvbi1kZXRhaWwifSx7ImFjdGlvbnMiOlsic3Vic2NyaWJlIl0sInRvcGljcyI6W3sibWF0Y2giOiIqIn1dLCJ0eXBlIjoiaHR0cHM6Ly9tZXJjdXJlLnJvY2tzL2F1dGhvcml6YXRpb24tZGV0YWlsIn1dLCJleHAiOjQxMDI0NDQ4MDAsImlzcyI6Imh0dHBzOi8vbG9jYWxob3N0In0.VO0-PRjJ2MGOrMk2HxlrBv217pB7hyLxLIQUGgSfyXs' \
  --data-urlencode 'topic=https://example.com/books/1' \
  --data-urlencode 'data={"status": "checked out"}'
```

`--insecure` skips certificate verification, needed here only because the dev hub's certificate is self-signed. Drop it once you point `curl` at a hub with a real certificate.

Keep the browser tab open while publishing. The message appears at the top of the list without a reload.

The bearer token is an OAuth 2.0 access token signed with the playground key `!ChangeThisMercureHubJWTSecretKey!` (header `{ "alg": "HS256", "typ": "at+jwt" }`), carrying:

```json
{
  "iss": "https://localhost",
  "aud": "https://localhost/.well-known/mercure",
  "exp": 4102444800,
  "authorization_details": [
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["publish"],
      "topics": [{ "match": "*" }]
    },
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["subscribe"],
      "topics": [{ "match": "*" }]
    }
  ]
}
```

The token grants access to every topic and expires in 2100. Use it only with this local playground. To generate a development token with the full RFC 9068 claims, run `caddy mercure-token --dev` in the container, or `./mercure mercure-token --dev` with the downloaded binary. See [Authorization](../concepts/authorization.md).

## Closing the Mercure EventSource connection

`EventSource` keeps an SSE stream open while the page is active. Single-page apps in particular should call `es.close()` when the component that opened the stream unmounts:

```javascript
useEffect(() => {
  const es = new EventSource(url);
  es.onmessage = (e) => console.log(e.data);
  return () => es.close();
}, [url]);
```

Close streams when their components unmount to avoid keeping unused subscriptions open.

## Mercure quickstart: publish/subscribe flow recap

```text
publisher -- POST /.well-known/mercure --> hub
subscriber -- GET /.well-known/mercure?match=... --> hub
subscriber <-- SSE updates ---------------------- hub
```

Your application publishes updates; the hub holds the subscriber connections. Publishers can run in an API server, a worker, or a serverless function.

## Don't want to manage a hub?

**[Mercure Cloud](https://mercure.rocks/pricing) runs it for you.** Take what you just built to production with managed hosting, automatic HTTPS, and a custom domain. Use your Cloud hub URL and its authorization settings in place of the local playground.

Need the hub on your own infrastructure? [Mercure Enterprise](../production/high-availability.md) adds clustering, shared transports, and direct support. Our [Managed On-Premise option](https://mercure.rocks/pricing) also covers deployment, monitoring, and updates.

## Next steps

- **Learn the protocol surface**: [Topics and matchers](../concepts/topics-and-matchers.md), [Authorization](../concepts/authorization.md).
- **Build something concrete**: the [LLM streaming](../use-cases/llm-token-streaming.md) and [AI agent progress](../use-cases/ai-agent-progress.md) guides show publisher and subscriber examples.
- **Move toward production**: [Configuration](../deployment/configuration.md), [Health checks](../production/health-monitoring.md), [Rolling updates](../production/rolling-updates.md).
