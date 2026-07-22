"use strict";

/* eslint-env browser */
/* global EventSourcePolyfill */

(function () {
  const origin = window.location.origin;
  const defaultTopic = document.URL + "demo/books/1.jsonld";
  const placeholderTopic = "https://example.com/my-private-topic";

  // RFC 9068 access token (typ: at+jwt, aud: the hub resource identifier).
  // Signed with `!ChangeThisMercureHubJWTSecretKey!`.
  //
  // {
  //   "aud": "https://localhost/.well-known/mercure",
  //   "exp": 4102444800,
  //   "authorization_details": [
  //     { "type": "mercure", "actions": ["publish"], "topics": [{ "match": "*" }] },
  //     {
  //       "type": "mercure",
  //       "actions": ["subscribe"],
  //       "topics": [
  //         { "match": "https://example.com/my-private-topic" },
  //         { "match": "https://example.com/demo/books/:id.jsonld", "match_type": "urlpattern" },
  //         { "match": "/.well-known/mercure/subscriptions{/:matchType}?{/:match}?{/:subscriber}?", "match_type": "urlpattern" }
  //       ],
  //       "payload": { "user": "https://example.com/users/dunglas", "remoteAddr": "127.0.0.1" }
  //     }
  //   ]
  // }
  const defaultJwt =
    "eyJhbGciOiJIUzI1NiIsInR5cCI6ImF0K2p3dCJ9.eyJhdWQiOiJodHRwczovL2xvY2FsaG9zdC8ud2VsbC1rbm93bi9tZXJjdXJlIiwiYXV0aG9yaXphdGlvbl9kZXRhaWxzIjpbeyJhY3Rpb25zIjpbInB1Ymxpc2giXSwidG9waWNzIjpbeyJtYXRjaCI6IioifV0sInR5cGUiOiJtZXJjdXJlIn0seyJhY3Rpb25zIjpbInN1YnNjcmliZSJdLCJwYXlsb2FkIjp7InJlbW90ZUFkZHIiOiIxMjcuMC4wLjEiLCJ1c2VyIjoiaHR0cHM6Ly9leGFtcGxlLmNvbS91c2Vycy9kdW5nbGFzIn0sInRvcGljcyI6W3sibWF0Y2giOiJodHRwczovL2V4YW1wbGUuY29tL215LXByaXZhdGUtdG9waWMifSx7Im1hdGNoIjoiaHR0cHM6Ly9leGFtcGxlLmNvbS9kZW1vL2Jvb2tzLzppZC5qc29ubGQiLCJtYXRjaF90eXBlIjoidXJscGF0dGVybiJ9LHsibWF0Y2giOiIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey86bWF0Y2hUeXBlfT97LzptYXRjaH0_ey86c3Vic2NyaWJlcn0_IiwibWF0Y2hfdHlwZSI6InVybHBhdHRlcm4ifV0sInR5cGUiOiJtZXJjdXJlIn1dLCJleHAiOjQxMDI0NDQ4MDB9.ZgpUCyCES-GCIP4_U6T5m0FJ4sN38QNDG4LAVhgSh1g";

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

  // lastEventIdQueryParameterName: the hub reads last_event_id, not the
  // polyfill's default lastEventId, so reconnects would replay from the start.
  const openEventSource = (url) => {
    const opts = { lastEventIdQueryParameterName: "last_event_id" };
    if ($settingsForm.authorization.value === "header") {
      opts.headers = { Authorization: `Bearer ${$settingsForm.jwt.value}` };
    } else {
      opts.withCredentials = true;
    }
    return new EventSourcePolyfill(url, opts);
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
    // Idempotent: replays re-deliver the same id.
    if (document.getElementById(s.id)) {
      return;
    }

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
      // The snapshot cursor is carried by the rel="mercure" Link header's
      // last-event-id attribute, not the JSON body.
      const lastEventId = (resp.headers.get("Link") ?? "").match(
        /rel="mercure".*?last-event-id="([^"]*)"/,
      )?.[1];
      const json = await resp.json();

      json.subscriptions.forEach(addSubscription);

      // Stream changes since the snapshot, so nothing is missed in between.
      const u = new URL($settingsForm.hubUrl.value);
      u.searchParams.append(
        "match_urlpattern",
        "/.well-known/mercure/subscriptions/:match_type/:match/:subscriber",
      );
      if (lastEventId) u.searchParams.append("last_event_id", lastEventId);

      subscriptionEventSource = openEventSource(u);

      // Subscription updates use the "mercure" SSE event type, not the default.
      subscriptionEventSource.addEventListener("mercure", function (e) {
        const s = JSON.parse(e.data);

        if (s.active) {
          addSubscription(s);
          return;
        }

        document.getElementById(s.id)?.remove();
      });
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
