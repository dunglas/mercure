const type = "https://chat.example.com/Message";
const topic = "https://chat.example.com/messages/{id}";
const { hubURL, userIRI, connectedUsers } = JSON.parse(
  document.getElementById("config").textContent
);

const iriToUserName = (iri) =>
  iri.replace(/^https:\/\/chat.example.com\/users\//, "");
document.getElementById("username").textContent = iriToUserName(userIRI);

const $messages = document.getElementById("messages");
const $userList = document.getElementById("userList");
let $userListUL = null;
let $messagesUL = null;

let userList = new Map(
  connectedUsers
    .reduce((acc, val) => {
      if (val !== userIRI) acc.push([val, true]);
      return acc;
    }, [])
    .sort()
);

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

  userList.forEach((v, userIRI) => {
    const li = document.createElement("li");
    li.append(document.createTextNode(iriToUserName(userIRI)));
    $userListUL.append(li);
  });
  $userList.append($userListUL);
};

updateUserListView();

const subscribeURL = new URL(hubURL);
subscribeURL.searchParams.append("Last-Event-ID", config.lastEventID);
subscribeURL.searchParams.append("topic", topic);
subscribeURL.searchParams.append(
  "topic",
  `https://mercure.rocks/subscriptions/${encodeURIComponent(
    topic
  )}/{subscriptionID}`
);

const es = new EventSource(subscribeURL, { withCredentials: true });
es.onmessage = ({ data }) => {
  const update = JSON.parse(data);

  switch (update["@type"]) {
    case type:
      displayMessage(update);
      return;
    case "https://mercure.rocks/Subscription":
      updateUserList(update);
      return;
    default:
      console.error("Unknown update type");
  }
};

const displayMessage = ({ user, message }) => {
  if (!$messagesUL) {
    $messagesUL = document.createElement("ul");

    $messages.innerText = "";
    $messages.append($messagesUL);
  }

  const li = document.createElement("li");
  li.append(document.createTextNode(`<${iriToUserName(user)}> ${message}`));
  $messagesUL.append(li);
};

const updateUserList = ({ active, subscribe }) => {
  const user = subscribe.find((u) =>
    u.startsWith("https://chat.example.com/users/")
  );
  if (user === userIRI) return;

  active ? userList.set(user, true) : userList.delete(user);

  userList = new Map([...userList.entries()].sort());

  updateUserListView();
};

document.querySelector("form").onsubmit = function (e) {
  e.preventDefault();

  const uid = window.crypto.getRandomValues(new Uint8Array(10)).join("");
  const iri = topic.replace("{id}", uid);

  const body = new URLSearchParams({
    data: JSON.stringify({
      "@type": type,
      "@id": iri,
      user: userIRI,
      message: this.elements.message.value,
    }),
    topic: iri,
    target: "https://chat.example.com/user",
  });
  fetch(hubURL, { method: "POST", body, credentials: "include" });
  this.elements.message.value = "";
  this.elements.message.focus();
};
