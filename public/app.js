'use strict';

const defaultTopic = window.location.origin + '/demo/books/1.jsonld';

// Set default values
document.addEventListener('DOMContentLoaded', function() {
    const topic = document.forms.discover.topic;
    if (!topic.value) topic.value = defaultTopic;
});

function getHubUrl(response) {
    const link = response.headers.get('Link');
    if (link) {
        const match = link.match(/<(.*)>.*rel="mercure".*/);
        if (match && match[1]) return match[1];
    }

    console.error('No rel="mercure" Link header provided.');
};

// Discover
document.forms.discover.onsubmit = function (e) {
    e.preventDefault();
    const { elements: { topic, body, jwt } } = this;
    const params = body.value || jwt.value ? `?${new URLSearchParams({body: body.value, jwt: jwt.value})}` : '';

    fetch(topic.value + params)
        .then(response => {
            // Set subscribe default values
            const subscribeUrl = document.forms.subscribe.url;
            if (!subscribeUrl.value) {
                const providedUrl = getHubUrl(response);
                if (providedUrl) {
                    subscribeUrl.value = new URL(providedUrl, topic.value);    
                }
            }

            const subscribeTopics = document.forms.subscribe.topics;
            if (!subscribeTopics.value) {
                if (topic.value === defaultTopic) {
                    subscribeTopics.value = `${topic.value}\n${window.location.origin}/demo/books/novels/{id}.jsonld`;
                } else {
                    subscribeTopics.value = topic.value;
                }
            }

            const publishForm = document.forms.publish;

            // Set publish default values
            const publishTopics = publishForm.topics;
            if (!publishTopics.value) {
                if (topic.value === defaultTopic) {
                    publishTopics.value = `${topic.value}\n${window.location.origin}/demo/books/novels/1.jsonld`;
                } else {
                    publishTopics.value = topic.value;
                }
            }
            const publishData = publishForm.data;
            if (!publishData.value && topic.value === defaultTopic) {
                publishData.value = JSON.stringify({'@id': defaultTopic, availability: 'http://schema.org/OutOfStock'});
            }

            return response.text();
        })
        .then(data => body.value = data)
        .catch(error => console.error(error))
}

// Subscribe
const template = document.querySelector('#update');
let ol, eventSource;
document.forms.subscribe.onsubmit = function (e) {
    e.preventDefault();
    eventSource && eventSource.close();

    const { elements: { topics, lastEventId } } = this;

    const topicList = topics.value.split("\n");
    const params = new URLSearchParams();
    topicList.forEach(topic => params.append('topic', topic));
    if (lastEventId) {
        params.append('Last-Event-ID', lastEventId.value);
    }

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
        console.error(e);
    };

    this.elements.unsubscribe.disabled = false;
};
document.forms.subscribe.elements.unsubscribe.onclick = function () {
    eventSource.close();
    this.disabled = true;
};

// Publish
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
        .catch(error => console.error(error))
};
