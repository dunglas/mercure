'use strict';

document.forms.publish.onsubmit = function (e) {
    e.preventDefault();
    const { action, method, elements: { topics, data, targets, jwt, id, type, retry } } = this;

    const body = new URLSearchParams({
        data: data.value,
        id: id.value,
        type: type.value,
        retry: retry.value,
    });

    topics.value.split("\n").forEach(topic => body.append('topic', topic));
    targets.value !== '' && targets.value.split("\n").forEach(target => body.append('target', target));

    fetch(action, { method, body, headers: { Authorization: `Bearer ${jwt.value}` } })
        .then(({ ok, statusText }) => !ok && alert(statusText));
};

const deleteCookie = () => document.cookie = 'mercureAuthorization=;expires=Thu, 01 Jan 1970 00:00:01 GMT;';

const template = document.querySelector('#update');
let ol, eventSource;
document.forms.subscribe.onsubmit = function (e) {
    e.preventDefault();
    eventSource && eventSource.close();

    const { elements: { topics, jwt, lastEventId } } = this;

    const topicList = topics.value.split("\n");
    const params = new URLSearchParams();
    topicList.forEach(topic => params.append('topic', topic));
    if (lastEventId) {
        params.append('Last-Event-ID', lastEventId.value);
    }

    jwt.value ? document.cookie = `mercureAuthorization=${encodeURIComponent(jwt.value)}` : deleteCookie();

    eventSource = new EventSource(`/subscribe?${params}`);
    eventSource.onmessage = function (e) {
        if (!ol) {
            ol = document.createElement('ol');
            ol.reversed = true;

            const updates = document.querySelector('#updates');
            updates.innerHTML = '';
            updates.appendChild(ol);
        }

        const li = document.importNode(template.content, true);
        li.querySelector('h1').textContent = e.lastEventId;
        li.querySelector('pre').textContent = e.data;
        ol.firstChild ? ol.insertBefore(li, ol.firstChild) : ol.appendChild(li);
    };
    eventSource.onerror = function (e) {
        console.log(e);
    };

    this.elements.unsubscribe.disabled = false;
};
document.forms.subscribe.elements.unsubscribe.onclick = function () {
    eventSource.close();
    deleteCookie();
    this.disabled = true;
};

document.forms.subscribe.elements.subscribe.click();
