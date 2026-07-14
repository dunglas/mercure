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
  //       { "match": "https://example.com/demo/books/:id.jsonld", "match_type": "urlpattern" },
  //       { "match": "/.well-known/mercure/subscriptions/:match_type/:match/:subscriber", "match_type": "urlpattern" }
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

  // openEventSource builds an EventSource-like object using the polyfill.
  // Cookie auth goes through withCredentials; header auth goes through the
  // Authorization header (unsupported by native EventSource in browsers).
  const openEventSource = (url) => {
    if ($settingsForm.authorization.value === "header") {
      return new EventSourcePolyfill(url, {
        headers: { Authorization: `Bearer ${$settingsForm.jwt.value}` },
      });
    }
    return new EventSourcePolyfill(url, { withCredentials: true });
  };

  // Subscribe
  const $updateTemplate = document.getElementById("update");
  let updateEventSource;
  $subscribeForm.onsubmit = function (e) {
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
      u.searchParams.append("last_event_id", lastEventId.value);
    }

    let ol = null;
    updateEventSource = openEventSource(u);

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
    const unsubscribeBtn = this.elements.unsubscribe;
    updateEventSource.onerror = error;
    unsubscribeBtn.disabled = false;
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

    // An update has exactly one topic: publish one update per line.
    const topicList = topics.value
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line.length > 0);

    try {
      for (const topic of topicList) {
        const body = new URLSearchParams({
          topic,
          data: data.value,
          id: id.value,
          type: type.value,
          retry: retry.value,
        });
        priv.checked && body.append("private", "on");

        const opt = { method: "POST", body };
        if ($settingsForm.authorization.value === "header") {
          opt.headers = { Authorization: `Bearer ${$settingsForm.jwt.value}` };
        }

        const resp = await fetch($settingsForm.hubUrl.value, opt);
        if (!resp.ok) throw new Error(resp.statusText);
      }
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
    // v9+ subscriptions expose match/match_type; deprecated ones expose topic.
    subscription.querySelector(".match").textContent =
      s.match !== undefined ? `${s.match_type || "exact"} ${s.match}` : s.topic;
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
      // covers every {match_type}/{match}/{subscriber} triple.
      const u = new URL($settingsForm.hubUrl.value);
      u.searchParams.append(
        "match_urlpattern",
        "/.well-known/mercure/subscriptions/:match_type/:match/:subscriber",
      );
      u.searchParams.append("last_event_id", json.last_event_id);

      subscriptionEventSource = openEventSource(u);

      subscriptionEventSource.onmessage = function (e) {
        const s = JSON.parse(e.data);

        if (s.active) {
          addSubscription(s);
          return;
        }

        document.getElementById(s.id).remove();
      };
      const unsubscribeBtn = $subscriptionsForm.elements.unsubscribe;
      subscriptionEventSource.onerror = error;
      unsubscribeBtn.disabled = false;
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
