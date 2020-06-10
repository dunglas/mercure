"use strict";

(function () {
  const origin = window.location.origin;
  const defaultTopic = origin + "/demo/books/1.jsonld";
  const placeholderTopic = "https://example.com/my-private-topic";
  const defaultJwt =
    "eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwie3NjaGVtZX06Ly97K2hvc3R9L2RlbW8vYm9va3Mve2lkfS5qc29ubGQiLCIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey90b3BpY317L3N1YnNjcmliZXJ9Il0sInBheWxvYWQiOnsidXNlciI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdXNlcnMvZHVuZ2xhcyIsInJlbW90ZUFkZHIiOiIxMjcuMC4wLjEifX19.z5YrkHwtkz3O_nOnhC_FP7_bmeISe3eykAkGbAl5K7c";

  const $updates = document.getElementById("updates");
  const $subscriptions = document.getElementById("subscriptions");
  const $settingsForm = document.forms.settings;
  const $discoverForm = document.forms.discover;
  const $subscribeForm = document.forms.subscribe;
  const $publishForm = document.forms.publish;
  const $subscriptionsForm = document.forms.subscriptions;

  const error = (e) => {
    const message = e.toString();
    console.error(e);
    alert(message);
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
      2
    );
    $publishForm.data.value = JSON.stringify(
      {
        "@id": defaultTopic,
        availability: "https://schema.org/OutOfStock",
      },
      null,
      2
    );

    document.getElementById(
      "subscribeTopicsExamples"
    ).textContent = `${origin}/demo/novels/{id}.jsonld
${defaultTopic}
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
      if (subscribeTopics.value === placeholderTopic)
        subscribeTopics.value = topic.value;

      // Set publish default values
      const publishTopics = $publishForm.topics;
      if (publishTopics.value === placeholderTopic)
        publishTopics.value = topic.value;

      const text = await resp.text();
      body.value = text;
    } catch (e) {
      error(e);
    }
  };

  // Subscribe
  const $updateTemplate = document.getElementById("update");
  let updateEventSource;
  $subscribeForm.onsubmit = function (e) {
    e.preventDefault();

    updateEventSource && updateEventSource.close();
    $updates.textContent = "No updates pushed yet.";

    const {
      elements: { topics, lastEventId },
    } = this;

    const topicList = topics.value.split("\n");
    const u = new URL($settingsForm.hubUrl.value);
    topicList.forEach((topic) => u.searchParams.append("topic", topic));
    if (lastEventId.value)
      u.searchParams.append("Last-Event-ID", lastEventId.value);

    let ol = null;
    if ($settingsForm.authorization.value === "header")
      updateEventSource = new EventSourcePolyfill(u, {
        headers: {
          Authorization: `Bearer ${$settingsForm.jwt.value}`,
        },
      });
    else updateEventSource = new EventSource(u);

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
    updateEventSource.onerror = console.log;
    this.elements.unsubscribe.disabled = false;
  };
  $subscribeForm.elements.unsubscribe.onclick = function (e) {
    e.preventDefault();

    updateEventSource.close();
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
    if ($settingsForm.authorization.value === "header")
      opt.headers = { Authorization: `Bearer ${$settingsForm.jwt.value}` };

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
      true
    );
    subscription.querySelector("div").setAttribute("id", s.id);
    subscription.querySelector(".card-header-title").textContent = s.id;
    subscription.querySelector(".topic").textContent = s.topic;
    subscription.querySelector(".subscriber").textContent = s.subscriber;
    subscription.querySelector("code").textContent = JSON.stringify(
      s.payload,
      null,
      2
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
        opt
      );
      if (!resp.ok) throw new Error(resp.statusText);
      const json = await resp.json();

      json.subscriptions.forEach(addSubscription);

      const u = new URL($settingsForm.hubUrl.value);
      u.searchParams.append(
        "topic",
        "/.well-known/mercure/subscriptions{/topic}{/subscriber}"
      );
      u.searchParams.append("Last-Event-ID", json.lastEventID);

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
      subscriptionEventSource.onerror = console.log;

      $subscriptionsForm.elements.unsubscribe.disabled = false;
    } catch (e) {
      error(e);
      return;
    }
  };
  $subscriptionsForm.elements.unsubscribe.onclick = function (e) {
    e.preventDefault();

    subscriptionEventSource.close();
    this.disabled = true;
    $subscriptions.textContent = "";
  };
})();
