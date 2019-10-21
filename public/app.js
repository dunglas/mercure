'use strict';

(function () {
    const origin = window.location.origin;
    const defaultTopic = origin + '/demo/books/1.jsonld';
    const defaultJwt = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6WyJmb28iLCJiYXIiXSwicHVibGlzaCI6WyJmb28iXX19.afLx2f2ut3YgNVFStCx95Zm_UND1mZJ69OenXaDuZL8';

    const settingsForm = document.forms.settings;
    const discoverForm = document.forms.discover;
    const subscribeForm = document.forms.subscribe;
    const publishForm = document.forms.publish;

    function error(e) {
        const message = e.toString();
        console.error(e);
        alert(message === '[object Event]' ? 'EventSource error' : message);
    }

    function getHubUrl(response) {
        const link = response.headers.get('Link');
        if (link) {
            const match = link.match(/<(.*)>.*rel="mercure".*/);
            if (match && match[1]) return match[1];
        }

        error('No rel="mercure" Link header provided.');
    };

    // Set default values
    document.addEventListener('DOMContentLoaded', function () {
        settingsForm.hubUrl.value = origin + '/.well-known/mercure';
        settingsForm.jwt.value = defaultJwt;

        discoverForm.topic.value = defaultTopic;
        discoverForm.body.value = JSON.stringify({
            '@id': defaultTopic,
            availability: 'https://schema.org/InStock',
        }, null, 2);
        publishForm.data.value = JSON.stringify({
            '@id': defaultTopic,
            availability: 'https://schema.org/OutOfStock',
        }, null, 2);

        document.getElementById('subscribeTopicsExamples').innerText = `${origin}/demo/novels/{id}.jsonld
${defaultTopic}
foo`;
    });

    // Discover
    discoverForm.onsubmit = function (e) {
        e.preventDefault();
        const { elements: { topic, body } } = this;
        const jwt = settingsForm.jwt.value;

        const url = new URL(topic.value);
        if (body.value) url.searchParams.append('body', body.value);
        if (jwt) url.searchParams.append('jwt', jwt);

        fetch(url)
            .then(response => {
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
            .then(data => body.value = data)
            .catch(e => error(e))
    }

    // Subscribe
    const template = document.querySelector('#update');
    let ol, eventSource;
    subscribeForm.onsubmit = function (e) {
        e.preventDefault();

        if (settingsForm.authorization.value === 'header') {
            alert('EventSource do not support setting HTTP headers.');
            return;
        }

        eventSource && eventSource.close();

        const { elements: { topics, lastEventId } } = this;

        const topicList = topics.value.split("\n");
        const u = new URL(settingsForm.hubUrl.value);
        topicList.forEach(topic => u.searchParams.append('topic', topic));
        if (lastEventId.value) u.searchParams.append('Last-Event-ID', lastEventId.value);

        eventSource = new EventSource(u);
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
        eventSource.onerror = error;
        this.elements.unsubscribe.disabled = false;
    };
    subscribeForm.elements.unsubscribe.onclick = function () {
        eventSource.close();
        this.disabled = true;
    };

    // Publish
    publishForm.onsubmit = function (e) {
        e.preventDefault();
        const { elements: { topics, data, targets, id, type, retry } } = this;

        const body = new URLSearchParams({
            data: data.value,
            id: id.value,
            type: type.value,
            retry: retry.value,
        });

        topics.value.split("\n").forEach(topic => body.append('topic', topic));
        targets.value !== '' && targets.value.split("\n").forEach(target => body.append('target', target));

        const opt = { method: 'POST', body };
        if (settingsForm.authorization.value === 'header') opt.headers = { Authorization: `Bearer ${settingsForm.jwt.value}` };

        fetch(settingsForm.hubUrl.value, opt)
            .then(response => {
                if (!response.ok) throw new Error(response.statusText);
            })
            .catch(e => error(e))
            ;
    };
})();
