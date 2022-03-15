import { test, expect } from '@playwright/test';
import { randomBytes } from "crypto";

function randomString() {
  return randomBytes(20).toString('base64url');
}

test.beforeEach(async ({ page }) => await page.goto('/'));

test.describe('Publish update', () => {
  const randomStrings: string[] = [];
  for (let i = 0; i < 5; i++)
    randomStrings[i] = randomString();

  type Data = {name: string, updateTopics: string[], topicSelectors: string[], mustBeReceived: boolean, updateID?: string};

  const dataset: Data[] = [
    {name: 'raw string', updateTopics: [randomStrings[0]], topicSelectors: [randomStrings[0]], mustBeReceived: true},
    {name: 'multiple topics', updateTopics: [randomString(), randomStrings[1]], topicSelectors: [randomStrings[1]], mustBeReceived: true},
    {name: 'multiple topic selectors', updateTopics: [randomStrings[2]], topicSelectors: ['foo', randomStrings[2]], mustBeReceived: true},
    {name: 'URI', updateTopics: [`https://example.net/foo/${randomStrings[3]}`], topicSelectors: [`https://example.net/foo/${randomStrings[3]}`], mustBeReceived: true},
    {name: 'URI template', updateTopics: [`https://example.net/foo/${randomStrings[4]}`], topicSelectors: ['https://example.net/foo/{random}'], mustBeReceived: true},
  ];

  for (const data of dataset) {
    test(data.name, async ({ page }) => {
      data.updateID = `id-${JSON.stringify(data.updateTopics)}`;

      const { contentType, status, body } = await page.evaluate(async (data) => {
        let resolveReady: Function;
        const ready = new Promise((resolve) => {
          resolveReady = resolve;
        });

        let resolveReceived: Function;
        const received = new Promise((resolve) => {
          resolveReceived = resolve;
        });

        const url = new window.URL('/.well-known/mercure', window.origin);
        data.topicSelectors.forEach(topicSelector => url.searchParams.append('topic', topicSelector));

        const event = new window.URLSearchParams();
        data.updateTopics.forEach(updateTopic => event.append('topic', updateTopic));
        event.set('id', data.updateID);
        event.set('data', `data for

        ${data.name}`);

        console.log(event.get('id'), event.get('data'));

        const es = new EventSource(url);
        es.onopen = () => resolveReady(true);
        es.onmessage = (e) => {
          console.log('received');
          if (
            e.type === 'message' &&
            e.lastEventId === event.get('id') &&
            e.data === event.get('data')
          )
            resolveReceived(true);
        };

        await ready; // Wait for the EventSource to be ready
        const resp = await fetch('/.well-known/mercure', {
          method: 'POST',
          headers: { 'Authorization': 'Bearer eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiKiJdfX0.Ws4gtnaPtM-R2-z9DnH-laFu5lDZrMnmyTpfU8uKyQo' },
          body: event,
        });

        await received; // Wait for the event to be received

        return {
          contentType: resp.headers.get('Content-Type'),
          status: resp.status,
          body: await resp.text(),
        };
      }, data);

      expect(contentType).toMatch(/^text\/plain(?:$|;.*)/);
      expect(status).toBe(200);
      expect(body).toBe(data.updateID); // TODO: this feature is optional, add a flag to disable this check
    });
  }
});
