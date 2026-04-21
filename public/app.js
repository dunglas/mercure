"use strict";

/* eslint-env browser */
/* global EventSourcePolyfill */

(function () {
  const origin = window.location.origin;
  const defaultTopic = document.URL + "demo/books/1.jsonld";
  const placeholderTopic = "https://example.com/my-private-topic";

  // Object-form JWT (v9+ of the protocol). Signed with
  // `!ChangeThisMercureHubJWTSecretKey!`.
  //
  // {
  //   "mercure": {
  //     "publish":  [{ "match": "*" }],
  //     "subscribe": [
  //       { "match": "https://example.com/my-private-topic" },
  //       { "match": "https://example.com/demo/books/:id.jsonld", "matchType": "URLPattern" },
  //       { "match": "/.well-known/mercure/subscriptions/:matchType/:match/:subscriber", "matchType": "URLPattern" }
  //     ],
  //     "payload": { "user": "https://example.com/users/dunglas", "remoteAddr": "127.0.0.1" }
  //   }
  // }
  const defaultJwt =
    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlt7Im1hdGNoIjoiKiJ9XSwic3Vic2NyaWJlIjpbeyJtYXRjaCI6Imh0dHBzOi8vZXhhbXBsZS5jb20vbXktcHJpdmF0ZS10b3BpYyJ9LHsibWF0Y2giOiJodHRwczovL2V4YW1wbGUuY29tL2RlbW8vYm9va3MvOmlkLmpzb25sZCIsIm1hdGNoVHlwZSI6IlVSTFBhdHRlcm4ifSx7Im1hdGNoIjoiLy53ZWxsLWtub3duL21lcmN1cmUvc3Vic2NyaXB0aW9ucy86bWF0Y2hUeXBlLzptYXRjaC86c3Vic2NyaWJlciIsIm1hdGNoVHlwZSI6IlVSTFBhdHRlcm4ifV0sInBheWxvYWQiOnsicmVtb3RlQWRkciI6IjEyNy4wLjAuMSIsInVzZXIiOiJodHRwczovL2V4YW1wbGUuY29tL3VzZXJzL2R1bmdsYXMifX19.-I_LuyEjpjZKSfFI-4BstvrLzdCNslsSjHfR5RX0PcM";

  const $updates = document.getElementById("updates");
  const $subscriptions = document.getElementById("subscriptions");
  const $settingsForm = document.forms.settings;
  const $discoverForm = document.forms.discover;
  const $subscribeForm = document.forms.subscribe;
  const $publishForm = document.forms.publish;
  const $subscriptionsForm = document.forms.subscriptions;

  const error = (e) => {
    if (!e.error || e.error.message?.includes?.("Reconnecting")) {
      // Silent reconnecting messages from the polyfill

      console.log("Connection closed, reconnecting...", e);

      return;
    }

    console.log(e);

    if (e.toString !== Object.prototype.toString) {
      // Display relevant error message
      alert(e.toString());

      return;
    }

    if (e.statusText) {
      // Special handling of errors from the polyfill
      alert(e.statusText);

      return;
    }

    alert("An error occurred, details have been logged.");
  };

  const getHubUrl = (resp) => {
    const link = resp.headers.get("Link");
    if (!link) {
      error('No rel="mercure" Link header provided.');
    }

    const match = link.match(/<(.*)>.*rel="mercure".*/);
    if (match && match[1]) return match[1];
  };

  // matcherTypeLabel maps a match* query parameter name to the human-readable
  // name used in 501-unsupported alerts.
  const matcherTypeLabel = {
    match: "Exact",
    matchURLPattern: "URLPattern",
    matchRegexp: "Regexp",
    matchURITemplate: "URITemplate",
    matchCEL: "CEL",
  };

  // unsupportedMatcherAlert tells the operator how to enable a matcher type
  // when the hub answered 501 Not Implemented to a subscribe or subscription
  // request.
  const unsupportedMatcherAlert = (paramName) => {
    const label = matcherTypeLabel[paramName] ?? paramName;
    alert(
      `The hub does not support the "${label}" matcher type (501 Not Implemented).\n\n` +
        "Enable it in your hub configuration. Example Caddyfile directive:\n\n" +
        "    mercure {\n" +
        "        matcher_types exact urlpattern regexp uritemplate cel\n" +
        "    }\n\n" +
        'For the JSON config, set "matcher_types" on the module.',
    );
  };

  // Set default values
  document.addEventListener("DOMContentLoaded", () => {
    $settingsForm.hubUrl.value = origin + "/.well-known/mercure";
    $settingsForm.jwt.value = defaultJwt;

    $discoverForm.topic.value = defaultTopic;
    $discoverForm.body.value = JSON.stringify(
      {
        "@id": defaultTopic,
        availability: "https://schema.org/InStock",
      },
      null,
      2,
    );
    $publishForm.data.value = JSON.stringify(
      {
        "@id": defaultTopic,
        availability: "https://schema.org/OutOfStock",
      },
      null,
      2,
    );

    document.getElementById("subscribeTopicsExamples").textContent =
      `${defaultTopic}
${document.URL}demo/novels/:id.jsonld   (URL Pattern)
${document.URL}demo/books/{id}.jsonld   (URI Template)
^https://example\\.com/chapters/[0-9]+$ (Regexp)
topics.all(t, t.startsWith("https://example.com/")) (CEL)
foo`;
  });

  // Discover
  $discoverForm.onsubmit = async function (e) {
    e.preventDefault();
    const {
      elements: { topic, body },
    } = this;
    const jwt = $settingsForm.jwt.value;

    const url = new URL(topic.value);
    if (body.value) url.searchParams.append("body", body.value);
    if (jwt) url.searchParams.append("jwt", jwt);

    try {
      const resp = await fetch(url);
      if (!resp.ok) throw new Error(resp.statusText);

      // Set hub default
      const hubUrl = getHubUrl(resp);
      if (hubUrl) $settingsForm.hubUrl.value = new URL(hubUrl, topic.value);

      const subscribeTopics = $subscribeForm.topics;
      if (subscribeTopics.value === placeholderTopic) {
        subscribeTopics.value = topic.value;
      }

      // Set publish default values
      const publishTopics = $publishForm.topics;
      if (publishTopics.value === placeholderTopic) {
        publishTopics.value = topic.value;
      }

      body.value = await resp.text();
    } catch (e) {
      error(e);
    }
  };

  // preflightSubscribe does a throw-away GET against the subscribe URL so we
  // can inspect the HTTP status before opening an EventSource. EventSource
  // itself doesn't expose the status code; probing beforehand is the only
  // way to detect a 501 "Not Implemented" for an unregistered matcher type.
  const preflightSubscribe = async (url, authHeaders) => {
    const controller = new AbortController();
    const resp = await fetch(url, {
      headers: authHeaders,
      credentials: "same-origin",
      signal: controller.signal,
    });
    // Status + headers are available as soon as the promise resolves; abort
    // the body stream immediately to let the server close the connection.
    controller.abort();
    return resp.status;
  };

  // Subscribe
  const $updateTemplate = document.getElementById("update");
  let updateEventSource;
  $subscribeForm.onsubmit = async function (e) {
    e.preventDefault();

    updateEventSource && updateEventSource.close();
    $updates.textContent = "No updates pushed yet.";

    const {
      elements: { topics, matcherType, lastEventId },
    } = this;

    const paramName = matcherType.value;
    const u = new URL($settingsForm.hubUrl.value);
    topics.value
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line.length > 0)
      .forEach((pattern) => u.searchParams.append(paramName, pattern));
    if (lastEventId.value) {
      u.searchParams.append("lastEventID", lastEventId.value);
    }

    const useHeader = $settingsForm.authorization.value === "header";
    const authHeaders = useHeader
      ? { Authorization: `Bearer ${$settingsForm.jwt.value}` }
      : undefined;

    try {
      const status = await preflightSubscribe(u, authHeaders);
      if (status === 501) {
        unsupportedMatcherAlert(paramName);
        return;
      }
      if (status !== 200) {
        alert(`Subscribe failed: HTTP ${status}`);
        return;
      }
    } catch (err) {
      error(err);
      return;
    }

    let ol = null;
    if (useHeader) {
      updateEventSource = new EventSourcePolyfill(u, { headers: authHeaders });
    } else {
      updateEventSource = new EventSource(u);
    }

    updateEventSource.onmessage = function (e) {
      if (!ol) {
        ol = document.createElement("ol");
        ol.reversed = true;

        $updates.textContent = "";
        $updates.appendChild(ol);
      }

      const li = document.importNode($updateTemplate.content, true);
      li.querySelector("h2").textContent = e.lastEventId;
      li.querySelector("pre").textContent = e.data;
      ol.firstChild ? ol.insertBefore(li, ol.firstChild) : ol.appendChild(li);
    };
    updateEventSource.onerror = error;
    this.elements.unsubscribe.disabled = false;
  };
  $subscribeForm.elements.unsubscribe.onclick = function (e) {
    e.preventDefault();

    updateEventSource && updateEventSource.close();
    this.disabled = true;
    $updates.textContent = "Unsubscribed.";
  };

  // Publish
  $publishForm.onsubmit = async function (e) {
    e.preventDefault();
    const {
      elements: { topics, data, priv, id, type, retry },
    } = this;

    const body = new URLSearchParams({
      data: data.value,
      id: id.value,
      type: type.value,
      retry: retry.value,
    });

    topics.value.split("\n").forEach((topic) => body.append("topic", topic));
    priv.checked && body.append("private", "on");

    const opt = { method: "POST", body };
    if ($settingsForm.authorization.value === "header") {
      opt.headers = { Authorization: `Bearer ${$settingsForm.jwt.value}` };
    }

    try {
      const resp = await fetch($settingsForm.hubUrl.value, opt);
      if (!resp.ok) throw new Error(resp.statusText);
    } catch (e) {
      error(e);
    }
  };

  // Subscriptions
  const $subscriptionTemplate = document.getElementById("subscription");
  let subscriptionEventSource;

  const addSubscription = (s) => {
    const subscription = document.importNode(
      $subscriptionTemplate.content,
      true,
    );
    subscription.querySelector("div").setAttribute("id", s.id);
    subscription.querySelector(".card-header-title").textContent = s.id;
    // v9+ subscriptions expose match/matchType; legacy ones expose topic.
    subscription.querySelector(".match").textContent =
      s.match !== undefined ? `${s.matchType || "Exact"} ${s.match}` : s.topic;
    subscription.querySelector(".subscriber").textContent = s.subscriber;
    subscription.querySelector("code").textContent = JSON.stringify(
      s.payload,
      null,
      2,
    );
    $subscriptions.appendChild(subscription);
  };

  $subscriptionsForm.onsubmit = async (e) => {
    e.preventDefault();

    subscriptionEventSource && subscriptionEventSource.close();
    $subscriptions.textContent = "";

    try {
      const opt =
        $settingsForm.authorization.value === "header"
          ? { headers: { Authorization: `Bearer ${$settingsForm.jwt.value}` } }
          : undefined;
      const resp = await fetch(
        `${$settingsForm.hubUrl.value}/subscriptions`,
        opt,
      );
      if (!resp.ok) throw new Error(resp.statusText);
      const json = await resp.json();

      json.subscriptions.forEach(addSubscription);

      // Subscribe to subscription events using a URL-pattern matcher that
      // covers every {matchType}/{match}/{subscriber} triple.
      const u = new URL($settingsForm.hubUrl.value);
      u.searchParams.append(
        "matchURLPattern",
        "/.well-known/mercure/subscriptions/:matchType/:match/:subscriber",
      );
      u.searchParams.append("lastEventID", json.lastEventID);

      const status = await preflightSubscribe(u, opt && opt.headers);
      if (status === 501) {
        unsupportedMatcherAlert("matchURLPattern");
        return;
      }
      if (status !== 200) {
        alert(`Subscribe to subscriptions failed: HTTP ${status}`);
        return;
      }

      if (opt) subscriptionEventSource = new EventSourcePolyfill(u, opt);
      else subscriptionEventSource = new EventSource(u);

      subscriptionEventSource.onmessage = function (e) {
        const s = JSON.parse(e.data);

        if (s.active) {
          addSubscription(s);
          return;
        }

        document.getElementById(s.id).remove();
      };
      subscriptionEventSource.onerror = error;

      $subscriptionsForm.elements.unsubscribe.disabled = false;
    } catch (e) {
      error(e);
    }
  };
  $subscriptionsForm.elements.unsubscribe.onclick = function (e) {
    e.preventDefault();

    subscriptionEventSource.close();
    this.disabled = true;
    $subscriptions.textContent = "";
  };
})();
