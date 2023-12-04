# Using Mercure and Hotwire to Stream Page Changes

> [Hotwire](https://hotwire.dev) is an alternative approach to building modern web applications without using much JavaScript by sending HTML instead of JSON over the wire.

Hotwire comes with a handy feature called [Turbo Streams](https://turbo.hotwire.dev/handbook/streams).
Turbo Streams allow servers to push page changes to connected clients in real-time.

Using Mercure to power a Turbo Stream is straightforward, and doesn't require any external dependency:

```javascript
import { connectStreamSource } from "@hotwired/turbo";

// The "topic" parameter can be any string or URI
const es = new EventSource("https://example.com/.well-known/mercure?topic=my-stream");
connectStreamSource(es);
```

The native [`EventSource` class](https://developer.mozilla.org/en-US/docs/Web/API/EventSource) is used to connect to the [Server-Sent Events (SSE)](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events) endpoint exposed by [the Mercure hub](../hub/install.md).

## Broadcasting Page Changes With Mercure

To broadcast messages through Turbo Streams, simply send a `POST` HTTP request to [the Mercure hub](../hub/install.md):

```console
curl \
  -H 'Authorization: Bearer <snip>' \
  -d 'topic=my-stream' \
  -d 'data=<turbo-stream action=...' \
  -X POST \
  https://example.com/.well-known/mercure
```

* `topic` must be the same topic we used in the JavaScript code ;
* `data` contains the [Turbo Streams messages](https://turbo.hotwire.dev/handbook/streams#stream-messages-and-actions) to broadcast ;
* the `Authorization` header must contain [a valid publisher JWT](../../spec/mercure.md#publication).

[Other Mercure features](../../spec/mercure.md#publication), including broadcasting private updates to authorized subscribers, are also supported.

In this example, we use `curl` but any HTTP client or [Mercure client library](awesome.md#client-libraries) will work.

## Disconnecting From a Stream

To disconnect the stream source, use the `disconnectStreamSource()` function:

```javascript
import { disconnectStreamSource } from "@hotwired/turbo";

disconnectStreamSource(es);
```

## Creating a Stimulus Controller

Mercure also plays very well with [the Stimulus framework](https://stimulus.hotwire.dev/) (another component of Hotwire).

In the following example, we create [a Stimulus controller](https://stimulus.hotwire.dev/handbook/hello-stimulus#controllers-bring-html-to-life) to connect to the SSE stream exposed by the Mercure hub when an HTML element is created, and to disconnect when it is destroyed:

```javascript
// turbo_stream_controller.js
import { Controller } from "stimulus";
import { connectStreamSource, disconnectStreamSource } from "@hotwired/turbo";

export default class extends Controller {
  static values = { url: String };

  connect() {
    this.es = new EventSource(this.urlValue);
    connectStreamSource(this.es);
  }

  disconnect() {
    this.es.close();
    disconnectStreamSource(this.es);
  }
}
```

```html
<div data-controller="turbo-stream" data-turbo-stream-url-value="https://example.com/.well-known/mercure?topic=my-stream">
  <!-- ... -->
</div>
```
