"use strict";

(function () {
  const origin = window.location.origin;
  const defaultTopic = origin + "/demo/books/1.jsonld";
  const defaultJwt =
    "eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwie3NjaGVtZX06Ly97K2hvc3R9L2RlbW8vYm9va3Mve2lkfS5qc29ubGQiLCIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey90b3BpY317L3N1YnNjcmliZXJ9Il0sInBheWxvYWQiOnsidXNlciI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdXNlcnMvZHVuZ2xhcyIsInJlbW90ZUFkZHIiOiIxMjcuMC4wLjEifX19.z5YrkHwtkz3O_nOnhC_FP7_bmeISe3eykAkGbAl5K7c";

  const updates = document.querySelector("#updates");
  const settingsForm = document.forms.settings;
  const discoverForm = document.forms.discover;
  const subscribeForm = document.forms.subscribe;
  const publishForm = document.forms.publish;

  function error(e) {
    const message = e.toString();
    console.error(e);
    alert(message === "[object Event]" ? "EventSource error" : message);
  }

  function getHubUrl(response) {
    const link = response.headers.get("Link");
    if (link) {
      const match = link.match(/<(.*)>.*rel="mercure".*/);
      if (match && match[1]) return match[1];
    }

    error('No rel="mercure" Link header provided.');
  }

  // Set default values
  document.addEventListener("DOMContentLoaded", function () {
    settingsForm.hubUrl.value = origin + "/.well-known/mercure";
    settingsForm.jwt.value = defaultJwt;

    discoverForm.topic.value = defaultTopic;
    discoverForm.body.value = JSON.stringify(
      {
        "@id": defaultTopic,
        availability: "https://schema.org/InStock",
      },
      null,
      2
    );
    publishForm.data.value = JSON.stringify(
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
  discoverForm.onsubmit = function (e) {
    e.preventDefault();
    const {
      elements: { topic, body },
    } = this;
    const jwt = settingsForm.jwt.value;

    const url = new URL(topic.value);
    if (body.value) url.searchParams.append("body", body.value);
    if (jwt) url.searchParams.append("jwt", jwt);

    fetch(url)
      .then((response) => {
        if (!response.ok) throw new Error(response.statusText);

        // Set hub default
        const hubUrl = getHubUrl(response);
        if (hubUrl) settingsForm.hubUrl.value = new URL(hubUrl, topic.value);

        const subscribeTopics = subscribeForm.topics;
        if (!subscribeTopics.value) subscribeTopics.value = topic.value;

        // Set publish default values
        const publishTopics = publishForm.topics;
        if (!publishTopics.value) {
          publishTopics.value = topic.value;
        }

        return response.text();
      })
      .then((data) => (body.value = data))
      .catch((e) => error(e));
  };

  // Subscribe
  const template = document.querySelector("#update");
  let eventSource;
  subscribeForm.onsubmit = function (e) {
    e.preventDefault();

    if (settingsForm.authorization.value === "header") {
      alert("EventSource do not support setting HTTP headers.");
      return;
    }

    eventSource && eventSource.close();
    updates.innerText = "No updates pushed yet.";

    const {
      elements: { topics, lastEventId },
    } = this;

    const topicList = topics.value.split("\n");
    const u = new URL(settingsForm.hubUrl.value);
    topicList.forEach((topic) => u.searchParams.append("topic", topic));
    if (lastEventId.value)
      u.searchParams.append("Last-Event-ID", lastEventId.value);

    let ol = null;
    eventSource = new EventSource(u);
    eventSource.onmessage = function (e) {
      if (!ol) {
        ol = document.createElement("ol");
        ol.reversed = true;
  
        updates.textContent = "";
        updates.appendChild(ol);
      }

      const li = document.importNode(template.content, true);
      li.querySelector("h1").textContent = e.lastEventId;
      li.querySelector("pre").textContent = e.data;
      ol.firstChild ? ol.insertBefore(li, ol.firstChild) : ol.appendChild(li);
    };
    eventSource.onerror = error;
    this.elements.unsubscribe.disabled = false;
  };
  subscribeForm.elements.unsubscribe.onclick = function () {
    eventSource.close();
    this.disabled = true;
    updates.innerText = "Unsubscribed.";
  };

  // Publish
  publishForm.onsubmit = function (e) {
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
    priv.checked && body.append("private", "on")

    const opt = { method: "POST", body };
    if (settingsForm.authorization.value === "header")
      opt.headers = { Authorization: `Bearer ${settingsForm.jwt.value}` };

    fetch(settingsForm.hubUrl.value, opt)
      .then((response) => {
        if (!response.ok) throw new Error(response.statusText);
      })
      .catch((e) => error(e));
  };
})();
