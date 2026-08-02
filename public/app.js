import { fetchEventSource } from "https://cdn.jsdelivr.net/npm/@microsoft/fetch-event-source@2/+esm";

const origin = window.location.origin;
const hubBase = `${origin}/.well-known/mercure`;
// The playground echo endpoints live at a root prefix, outside the reserved
// /.well-known/mercure namespace, so the resources they expose are valid topics.
const playgroundBase = `${origin}/playground/`;

// Example topics, one per matcher, kept out of the hub's reserved
// /.well-known/mercure namespace so they are publishable and subscribable.
const DEFAULTS = {
  match: "https://example.com/books/1",
  match_urlpattern: "https://example.com/books/:id",
};
const DEFAULT_TOPICS = new Set(Object.values(DEFAULTS));

const forms = document.forms;
const settings = forms.settings;
const $status = document.getElementById("status");
const $updates = document.getElementById("updates");
const $updateCount = document.getElementById("updateCount");
const $subscriptions = document.getElementById("subscriptions");
const updateTemplate = document.getElementById("update");
const subscriptionTemplate = document.getElementById("subscription");

// HTTPError carries an HTTP status through fetchEventSource's onopen/onerror so
// a 401/403 stops the stream and surfaces, instead of retrying forever.
class HTTPError extends Error {
  constructor(status, statusText) {
    super(statusText ? `${status} ${statusText}` : String(status));
    this.status = status;
  }
}

const setStatus = (state, label) => {
  $status.dataset.state = state;
  $status.textContent = label;
};

const report = (e) => {
  if (!e) return;
  console.error(e);
  window.alert(e instanceof Error ? e.message : String(e));
};

const useCookie = () => settings.authorization.value === "cookie";

const authHeaders = () =>
  !useCookie() && settings.jwt.value.trim()
    ? { Authorization: `Bearer ${settings.jwt.value.trim()}` }
    : {};

const credentials = () => (useCookie() ? "include" : "same-origin");

// The hub's auth cookie is HttpOnly, so JS can't set it; the playground endpoint does,
// from its ?jwt query. Plant it there before a cookie-authorized request (playground
// mode only — the option is hidden otherwise).
const ensureCookie = async () => {
  if (!useCookie()) return;

  await fetch(
    `${playgroundBase}cookie?jwt=${encodeURIComponent(settings.jwt.value.trim())}`,
    {
      credentials: "same-origin",
    },
  ).catch(() => {});
};

// openStream connects an SSE stream and returns its AbortController. onMessage
// receives every fetchEventSource message ({ id, event, data }). Transient
// network drops reconnect automatically (fetchEventSource resends the
// Last-Event-ID header, which the hub honors); an HTTP error is fatal.
const openStream = (url, onMessage) => {
  const controller = new AbortController();

  setStatus("off", "Connecting…");

  fetchEventSource(url.toString(), {
    signal: controller.signal,
    openWhenHidden: true,
    headers: authHeaders(),
    credentials: credentials(),
    async onopen(response) {
      if (response.ok) {
        setStatus("on", "Connected");
        return;
      }

      throw new HTTPError(response.status, response.statusText);
    },
    onmessage: onMessage,
    onerror(err) {
      if (err instanceof HTTPError) {
        setStatus("error", "Error");
        throw err; // fatal: stop retrying
      }

      setStatus("error", "Reconnecting…"); // transient: retry with default backoff
    },
    onclose() {
      setStatus("off", "Disconnected");
    },
  }).catch(report);

  return controller;
};

const getHubUrl = (response) => {
  const link = response.headers.get("Link");
  const match = link?.match(/<([^>]*)>[^,]*rel="mercure"/);

  return match?.[1];
};

// jwt.io deep-link, so a token can be inspected and edited then pasted back.
const updateJwtLink = () => {
  const token = settings.jwt.value.trim();
  document.getElementById("jwtInspect").href = token
    ? `https://jwt.io/#debugger-io?token=${encodeURIComponent(token)}`
    : "https://jwt.io";
};
settings.jwt.addEventListener("input", updateJwtLink);

// Segmented tabs write the picked value into the sibling hidden input; the
// matcher tabs also swap the example topic (exact vs URL pattern) unless edited.
document.querySelectorAll(".tabs").forEach((tabs) => {
  const input = tabs.parentElement.querySelector(
    `[name="${tabs.dataset.target}"]`,
  );

  tabs.querySelectorAll("button").forEach((button) => {
    button.onclick = () => {
      tabs
        .querySelectorAll("button")
        .forEach((b) => b.setAttribute("aria-selected", String(b === button)));
      input.value = button.dataset.value;

      if (input.name === "matcherType") {
        const topics = forms.subscribe.topics;
        if (DEFAULT_TOPICS.has(topics.value.trim())) {
          topics.value = DEFAULTS[button.dataset.value];
        }
      }
    };
  });
});

// Subscribe
let updatesController;
let updatesCount = 0;

const resetUpdates = () => {
  updatesCount = 0;
  $updateCount.textContent = "";
  $updates.replaceChildren(
    Object.assign(document.createElement("li"), {
      className: "empty",
      textContent: "Waiting for updates…",
    }),
  );
};

const prependUpdate = (event) => {
  $updates.querySelector(".empty")?.remove();

  const item = document.importNode(updateTemplate.content, true);
  item.querySelector(".id").textContent = event.id || "(no id)";
  item.querySelector(".time").textContent = new Date().toLocaleTimeString();
  item.querySelector("pre").textContent = event.data;
  $updates.insertBefore(item, $updates.firstChild);

  $updateCount.textContent = `${++updatesCount} received`;
};

forms.subscribe.onsubmit = async (e) => {
  e.preventDefault();
  updatesController?.abort();

  const { topics, matcherType, lastEventId } = e.target.elements;

  const url = new URL(settings.hubUrl.value);
  topics.value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean)
    .forEach((topic) => url.searchParams.append(matcherType.value, topic));
  if (lastEventId.value) {
    url.searchParams.append("last_event_id", lastEventId.value);
  }

  await ensureCookie();
  resetUpdates();
  updatesController = openStream(url, prependUpdate);
  e.target.elements.unsubscribe.disabled = false;
};

forms.subscribe.elements.unsubscribe.onclick = () => {
  updatesController?.abort();
  forms.subscribe.elements.unsubscribe.disabled = true;
  setStatus("off", "Disconnected");
  $updates.replaceChildren(
    Object.assign(document.createElement("li"), {
      className: "empty",
      textContent: "Unsubscribed.",
    }),
  );
};

// Publish
forms.publish.onsubmit = async (e) => {
  e.preventDefault();
  const { topics, data, priv, id, type, retry } = e.target.elements;

  const topicList = topics.value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);

  try {
    await ensureCookie();

    for (const topic of topicList) {
      const body = new URLSearchParams({
        topic,
        data: data.value,
        id: id.value,
        type: type.value,
        retry: retry.value,
      });
      if (priv.checked) body.append("private", "on");

      const response = await fetch(settings.hubUrl.value, {
        method: "POST",
        headers: authHeaders(),
        credentials: credentials(),
        body,
      });
      if (!response.ok)
        throw new HTTPError(response.status, response.statusText);
    }
  } catch (err) {
    report(err);
  }
};

// Discover (playground only)
forms.discover.onsubmit = async (e) => {
  e.preventDefault();
  const { topic, body } = e.target.elements;
  const jwt = settings.jwt.value.trim();

  const url = new URL(topic.value);
  if (body.value) url.searchParams.append("body", body.value);
  if (jwt) url.searchParams.append("jwt", jwt);

  try {
    const response = await fetch(url, { credentials: "same-origin" });
    if (!response.ok) throw new HTTPError(response.status, response.statusText);

    // Point the hub URL at whatever the resource advertises via rel="mercure".
    const hubUrl = getHubUrl(response);
    if (hubUrl) settings.hubUrl.value = new URL(hubUrl, topic.value);

    body.value = await response.text();
  } catch (err) {
    report(err);
  }
};

// Active subscriptions
let subscriptionsController;

const addSubscription = (s) => {
  if (document.getElementById(s.id)) return; // replays re-deliver the same id

  const node = document.importNode(subscriptionTemplate.content, true);
  node.querySelector(".sub").id = s.id;
  node.querySelector(".id").textContent = s.id;
  // Modern subscriptions expose match/match_type; deprecated ones expose topic.
  const modern = s.match !== undefined;
  node.querySelector(".match-type").textContent = modern
    ? s.match_type || "exact"
    : "topic";
  node.querySelector(".match").textContent = modern ? s.match : s.topic;
  node.querySelector(".subscriber").textContent = s.subscriber;

  const pre = node.querySelector("pre");
  if (s.payload === undefined) {
    node.querySelector(".payload-label").remove();
    pre.remove();
  } else {
    pre.textContent = JSON.stringify(s.payload, null, 2);
  }

  $subscriptions.appendChild(node);
};

forms.subscriptions.onsubmit = async (e) => {
  e.preventDefault();
  subscriptionsController?.abort();
  $subscriptions.replaceChildren();

  try {
    await ensureCookie();

    const response = await fetch(`${settings.hubUrl.value}/subscriptions`, {
      headers: authHeaders(),
      credentials: credentials(),
    });
    if (!response.ok) throw new HTTPError(response.status, response.statusText);

    // The snapshot cursor rides the rel="mercure" Link header, not the body.
    const lastEventId = response.headers
      .get("Link")
      ?.match(/rel="mercure".*?last-event-id="([^"]*)"/)?.[1];
    const json = await response.json();
    json.subscriptions.forEach(addSubscription);

    const url = new URL(settings.hubUrl.value);
    url.searchParams.append(
      "match_urlpattern",
      "/.well-known/mercure/subscriptions/:match_type/:match/:subscriber",
    );
    if (lastEventId) url.searchParams.append("last_event_id", lastEventId);

    subscriptionsController = openStream(url, (event) => {
      if (event.event !== "mercure") return; // subscription updates use this type

      const s = JSON.parse(event.data);
      if (s.active) addSubscription(s);
      else document.getElementById(s.id)?.remove();
    });
    e.target.elements.unsubscribe.disabled = false;
  } catch (err) {
    report(err);
  }
};

forms.subscriptions.elements.unsubscribe.onclick = () => {
  subscriptionsController?.abort();
  forms.subscriptions.elements.unsubscribe.disabled = true;
  $subscriptions.replaceChildren();
};

// Mode-aware initialization
const loadConfig = async () => {
  try {
    const response = await fetch("config.json");
    if (response.ok) return await response.json();
  } catch {
    /* no config endpoint: fall back to the prod-safe debugger. */
  }

  return { playground: false, anonymous: false, subscriptions: false };
};

const config = await loadConfig();

settings.hubUrl.value = hubBase;
updateJwtLink();

if (!config.subscriptions) {
  forms.subscriptions.elements.subscribe.disabled = true;
  document.getElementById("subscriptionsHint").textContent =
    "Disabled: this hub wasn't started with the subscriptions directive (MERCURE_EXTRA_DIRECTIVES=subscriptions to enable it).";
}

if (config.playground) {
  document.getElementById("mode").textContent = "Playground";
  document.getElementById("playground-banner").classList.remove("hidden");
  document.getElementById("discoverCard").classList.remove("hidden");

  forms.publish.data.value = JSON.stringify({ status: "available" }, null, 2);
  // The Discover topic is the hub's own playground endpoint: a stand-in resource that
  // advertises the hub via a rel="mercure" Link header.
  forms.discover.topic.value = `${playgroundBase}books/1.json`;
  forms.discover.body.value = JSON.stringify({ status: "available" }, null, 2);

  try {
    const response = await fetch("playground-token");
    if (response.ok) {
      settings.jwt.value = (await response.text()).trim();
      updateJwtLink();
      document.getElementById("jwtHint").textContent =
        "Prefilled with the hub's all-access playground token. Insecure — playground only.";
    }
  } catch {
    /* leave the field empty; the user can paste one. */
  }
} else {
  // The playground endpoint that plants the cookie exists only in playground mode.
  document.getElementById("cookieAuthLabel").classList.add("hidden");

  if (config.anonymous) {
    document.getElementById("jwtHint").innerHTML =
      "This hub allows anonymous subscription, so a token is optional here. It is required to publish, or to subscribe to private topics. Mint one with <code>caddy mercure-token</code>.";
  }
}
