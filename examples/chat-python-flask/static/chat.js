const type = "https://chat.example.com/Message";
const topic = "https://chat.example.com/messages/{id}";
const { hubURL, userIRI, connectedUsers } = JSON.parse(
  document.getElementById("config").textContent
);

const userList = new Map(
  connectedUsers.reduce((acc, val) => {
    acc.push([val, true]);
    return acc;
  }, [])
);

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
let ul = null;
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
  if (!ul) {
    ul = document.createElement("ul");

    const messages = document.getElementById("messages");
    messages.innerHTML = "";
    messages.append(ul);
  }

  const username = user.replace(/^https:\/\/chat.example.com\/users\//, "");
  const li = document.createElement("li");
  li.append(document.createTextNode(`<${username}> ${message}`));
  ul.append(li);
};

const updateUserList = ({ active, subscribe }) => {
  const user = subscribe.find((u) =>
    u.startsWith("https://chat.example.com/users/")
  );
  active ? userList.set(user, true) : userList.delete(user);

  console.log(userList);
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
