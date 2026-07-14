/* eslint-env browser */

const type = "https://chat.example.com/Message";
const { hubURL, messagePattern, subscriptionsTopic, presencePattern, username } =
  JSON.parse(document.getElementById("config").textContent);

document.getElementById("username").textContent = username;

const $messages = document.getElementById("messages");
const $messageTemplate = document.getElementById("message");
const $userList = document.getElementById("user-list");
const $onlineUserTemplate = document.getElementById("online-user");

let userList;
(async () => {
  const resp = await fetch(new URL(subscriptionsTopic, hubURL), {
    credentials: "include",
  });
  const subscriptionCollection = await resp.json();
  userList = new Map(
    subscriptionCollection.subscriptions
      .reduce((acc, { payload }) => {
        if (payload.username !== username) acc.push([payload.username, true]);
        return acc;
      }, [])
      .sort(),
  );
  updateUserListView();

  const subscribeURL = new URL(hubURL);
  subscribeURL.searchParams.append(
    "last_event_id",
    subscriptionCollection.last_event_id,
  );
  subscribeURL.searchParams.append("match_urlpattern", messagePattern);
  subscribeURL.searchParams.append("match_urlpattern", presencePattern);

  const es = new EventSource(subscribeURL, { withCredentials: true });
  es.onmessage = ({ data }) => {
    const update = JSON.parse(data);

    if (update["@type"] === type) {
      displayMessage(update);
      return;
    }

    if (update.type === "Subscription") {
      updateUserList(update);
      return;
    }

    console.warn("Received an unsupported update type", update);
  };
})();

const updateUserListView = () => {
  $userList.textContent = "";
  userList.forEach((_, username) => {
    const el = document.importNode($onlineUserTemplate.content, true);
    el.querySelector(".username").textContent = username;
    $userList.append(el);
  });
};

const displayMessage = ({ username, message }) => {
  const el = document.importNode($messageTemplate.content, true);
  el.querySelector(".username").textContent = username;
  el.querySelector(".msg").textContent = message;
  $messages.append(el);

  // scroll at the bottom when a new message is received
  $messages.scrollTop = $messages.scrollHeight;
};

const updateUserList = ({ active, payload }) => {
  if (username === payload.username) return;

  active
    ? userList.set(payload.username, true)
    : userList.delete(payload.username);
  userList = new Map([...userList.entries()].sort());

  updateUserListView();
};

document.querySelector("form").onsubmit = function (e) {
  e.preventDefault();

  const uid = window.crypto.getRandomValues(new Uint8Array(10)).join("");
  const messageTopic = messagePattern.replace(":id", uid);

  const body = new URLSearchParams({
    data: JSON.stringify({
      "@type": type,
      "@id": messageTopic,
      username,
      message: this.elements.message.value,
    }),
    topic: messageTopic,
    private: true,
  });
  fetch(hubURL, { method: "POST", body, credentials: "include" });
  this.elements.message.value = "";
  this.elements.message.focus();
};
