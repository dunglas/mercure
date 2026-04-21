import { test, expect } from "@playwright/test";
import { randomBytes } from "crypto";

function randomString() {
  return randomBytes(20).toString("hex");
}

test.beforeEach(async ({ page }) => await page.goto("/"));

test.describe("Publish update", () => {
  const randomStrings: string[] = Array.from({ length: 6 }, randomString);

  type Data = {
    name: string;
    updateTopics: string[];
    topicSelectors: string[];
    mustBeReceived: boolean;
    updateID?: string;
    private?: true;
  };

  const dataset: Data[] = [
    {
      name: "raw string",
      mustBeReceived: true,
      updateTopics: [randomStrings[0]],
      topicSelectors: [randomStrings[0]],
    },
    {
      name: "multiple topics",
      mustBeReceived: true,
      updateTopics: [randomString(), randomStrings[1]],
      topicSelectors: [randomStrings[1]],
    },
    {
      name: "multiple topic selectors",
      mustBeReceived: true,
      updateTopics: [randomStrings[2]],
      topicSelectors: ["foo", randomStrings[2]],
    },
    {
      name: "URI",
      mustBeReceived: true,
      updateTopics: [`https://example.net/foo/${randomStrings[3]}`],
      topicSelectors: [`https://example.net/foo/${randomStrings[3]}`],
    },
    {
      name: "URL pattern",
      mustBeReceived: true,
      updateTopics: [`https://example.net/foo/${randomStrings[4]}`],
      topicSelectors: ["https://example.net/foo/:random"],
    },
    {
      name: "URI template",
      mustBeReceived: true,
      updateTopics: [`https://example.net/foo/${randomStrings[5]}`],
      topicSelectors: ["https://example.net/foo/{random}"],
    },
    {
      name: "nonmatching raw string",
      mustBeReceived: false,
      updateTopics: [`will-not-match`],
      topicSelectors: ["another-name"],
    },
    {
      name: "nonmatching URI",
      mustBeReceived: false,
      updateTopics: [`https://example.net/foo/will-not-match`],
      topicSelectors: ["https://example.net/foo/another-name"],
    },
    {
      name: "nonmatching URL pattern",
      mustBeReceived: false,
      updateTopics: [`https://example.net/foo/will-not-match`],
      topicSelectors: ["https://example.net/bar/:var"],
    },
    {
      name: "nonmatching URI template",
      mustBeReceived: false,
      updateTopics: [`https://example.net/foo/will-not-match`],
      topicSelectors: ["https://example.net/bar/{var}"],
    },
    {
      name: "private raw string",
      mustBeReceived: false,
      private: true,
      updateTopics: [randomStrings[0]],
      topicSelectors: [randomStrings[0]],
    },
    {
      name: "private URI",
      mustBeReceived: false,
      private: true,
      updateTopics: [`https://example.net/foo/${randomStrings[3]}`],
      topicSelectors: [`https://example.net/foo/${randomStrings[3]}`],
    },
    {
      name: "private URL pattern",
      mustBeReceived: false,
      private: true,
      updateTopics: [`https://example.net/foo/${randomStrings[4]}`],
      topicSelectors: ["https://example.net/foo/:random"],
    },
    {
      name: "private URI template",
      mustBeReceived: false,
      private: true,
      updateTopics: [`https://example.net/foo/${randomStrings[5]}`],
      topicSelectors: ["https://example.net/foo/{random}"],
    },
  ];

  for (const data of dataset) {
    test(data.name, async ({ page }) => {
      page.on("console", (msg) => console.log(msg.text()));

      data.updateID = `id-${JSON.stringify(data.updateTopics)}`;

      const { received, contentType, status, body } = await page.evaluate(
        async (data) => {
          const receivedResult = Symbol("received");
          const notReceivedResult = Symbol("not received");

          let resolveReady: () => void;
          const ready = new Promise((resolve) => {
            resolveReady = () => resolve(true);
          });

          let resolveReceived: () => void;
          const received = new Promise((resolve) => {
            resolveReceived = () => resolve(receivedResult);
          });

          const timeout = new Promise((resolve) =>
            setTimeout(resolve, 2000, notReceivedResult),
          );

          const url = new window.URL("/.well-known/mercure", window.origin);
          // The v9 protocol replaced the single `topic` query parameter with
          // a family of `match*` parameters. Selectors carrying a `:param`
          // placeholder go through the URL Pattern matcher, those carrying
          // `{var}` through URI Template, everything else through the
          // exact-string matcher.
          const paramName = (selector: string): string => {
            if (/\/:[A-Za-z_]/.test(selector)) return "matchURLPattern";
            if (/\{[A-Za-z_]/.test(selector)) return "matchURITemplate";
            return "match";
          };
          data.topicSelectors.forEach((topicSelector) =>
            url.searchParams.append(paramName(topicSelector), topicSelector),
          );

          const event = new window.URLSearchParams();
          data.updateTopics.forEach((updateTopic) =>
            event.append("topic", updateTopic),
          );
          event.set("id", data.updateID);
          event.set(
            "data",
            `data for

        ${data.name}`,
          );
          if (data.private) event.set("private", "on");

          console.log(`data: ${JSON.stringify(data)}`);

          console.log(`creating EventSource: ${url}`);
          const es = new EventSource(url);
          console.log("EventSource created");

          es.onopen = () => {
            console.log("EventSource opened");
            resolveReady();
          };

          es.onmessage = (e) => {
            console.log(`EventSource event received: ${e.data}`);
            if (
              e.type === "message" &&
              e.lastEventId === event.get("id") &&
              e.data === event.get("data")
            ) {
              es.close();
              resolveReceived();
            }
          };

          await ready; // Wait for the EventSource to be ready

          console.log(`Creating POST request: ${event.toString()}`);
          const resp = await fetch(`/.well-known/mercure`, {
            method: "POST",
            headers: {
              // mercure.{publish,subscribe}: [{match: "*"}] — the v9 object
              // form of the deprecated ["*"] wildcard. The string form is
              // accepted only under WithProtocolVersionCompatibility.
              Authorization:
                "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlt7Im1hdGNoIjoiKiJ9XSwic3Vic2NyaWJlIjpbeyJtYXRjaCI6IioifV19fQ.0E7cOjh6kGKAPLC5mKzIvVIsV5j4hCNt9Ee0VY4kjqk",
            },
            body: event,
          });

          const id = await resp.text();
          console.log(
            `POST request done: ${JSON.stringify({ status: resp.status, id, event: event.toString() })}`,
          );

          switch (await Promise.race([received, timeout])) {
            case receivedResult:
              return {
                received: true,
                contentType: resp.headers.get("Content-Type"),
                status: resp.status,
                body: id,
              };

            case notReceivedResult:
              return {
                received: false,
              };
          }
        },
        data,
      );

      expect(received).toBe(data.mustBeReceived);

      if (data.mustBeReceived) {
        expect(contentType).toMatch(/^text\/plain(?:$|;.*)/);
        expect(status).toBe(200);
        if (process.env.CUSTOM_ID) expect(body).toBe(data.updateID);
      }
    });
  }
});
