const type = "https://chat.example.com/Message";
const { hubURL, messageTemplate, subscriptionsTopic, username } = JSON.parse(
  document.getElementById("config").textContent
);

document.getElementById("username").textContent = username;

const $messages = document.getElementById("messages");
const $userList = document.getElementById("userList");
let $userListUL = null;
let $messagesUL = null;

let userList, es;
(async () => {
  const resp = await fetch(new URL(subscriptionsTopic, hubURL), {
    credentials: "include",
  });
  const subscriptionCollection = await resp.json();
  userList = new Map(
    subscriptionCollection.subscriptions
      .reduce((acc, { payload }) => {
        if (payload.username != username) acc.push([payload.username, true]);
        return acc;
      }, [])
      .sort()
  );
  updateUserListView();

  const subscribeURL = new URL(hubURL);
  subscribeURL.searchParams.append(
    "Last-Event-ID",
    subscriptionCollection.lastEventID
  );
  subscribeURL.searchParams.append("topic", messageTemplate);
  subscribeURL.searchParams.append(
    "topic",
    `${subscriptionsTopic}{/subscriber}`
  );

  const es = new EventSource(subscribeURL, { withCredentials: true });
  es.onmessage = ({ data }) => {
    const update = JSON.parse(data);

    if (update["@type"] === type) {
      displayMessage(update);
      return;
    }

    if (update["type"] === "Subscription") {
      updateUserList(update);
      return;
    }

    console.warn("Received an unsupported update type", update);
  };
})();

const updateUserListView = () => {
  if (userList.size === 0) {
    $userList.textContent = "No other users";
    $messagesUL = null;
    return;
  }

  $userList.textContent = "";
  if ($userListUL === null) {
    $userListUL = document.createElement("ul");
  } else {
    $userListUL.textContent = "";
  }

  userList.forEach((_, username) => {
    const li = document.createElement("li");
    li.append(document.createTextNode(username));
    $userListUL.append(li);
  });
  $userList.append($userListUL);
};

const displayMessage = ({ username, message }) => {
  if (!$messagesUL) {
    $messagesUL = document.createElement("ul");

    $messages.innerText = "";
    $messages.append($messagesUL);
  }

  const li = document.createElement("li");
  li.append(document.createTextNode(`<${username}> ${message}`));
  $messagesUL.append(li);
};

const updateUserList = ({ active, payload }) => {
  if (username === payload.username) return;

  active ? userList.set(payload.username, true) : userList.delete(payload.username);
  userList = new Map([...userList.entries()].sort());

  updateUserListView();
};

document.querySelector("form").onsubmit = function (e) {
  e.preventDefault();

  const uid = window.crypto.getRandomValues(new Uint8Array(10)).join("");
  const messageTopic = messageTemplate.replace("{id}", uid);

  const body = new URLSearchParams({
    data: JSON.stringify({
      "@type": type,
      "@id": messageTopic,
      username: username,
      message: this.elements.message.value,
    }),
    topic: messageTopic,
    private: true,
  });
  fetch(hubURL, { method: "POST", body, credentials: "include" });
  this.elements.message.value = "";
  this.elements.message.focus();
};
