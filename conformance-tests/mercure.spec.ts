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
          // The `match` query parameter selects exact matching;
          // `match_urlpattern` selects the URL Pattern matcher. Selectors
          // carrying a `:param` placeholder go through the latter.
          const paramName = (selector: string): string => {
            if (/\/:[A-Za-z_]/.test(selector)) return "match_urlpattern";
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
              // RFC 9068 access token (typ at+jwt, aud the hub resource
              // identifier, iss the trusted issuer) granting publish and
              // subscribe on every topic via an authorization_details claim.
              Authorization:
                "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6ImF0K2p3dCJ9.eyJhdWQiOiJodHRwczovL2xvY2FsaG9zdC8ud2VsbC1rbm93bi9tZXJjdXJlIiwiaXNzIjoiaHR0cHM6Ly9sb2NhbGhvc3QiLCJhdXRob3JpemF0aW9uX2RldGFpbHMiOlt7InR5cGUiOiJodHRwczovL21lcmN1cmUucm9ja3MvYXV0aG9yaXphdGlvbi1kZXRhaWwiLCJhY3Rpb25zIjpbInB1Ymxpc2giXSwidG9waWNzIjpbeyJtYXRjaCI6IioifV19LHsidHlwZSI6Imh0dHBzOi8vbWVyY3VyZS5yb2Nrcy9hdXRob3JpemF0aW9uLWRldGFpbCIsImFjdGlvbnMiOlsic3Vic2NyaWJlIl0sInRvcGljcyI6W3sibWF0Y2giOiIqIn1dfV0sImV4cCI6NDEwMjQ0NDgwMH0.z_S8IP5WZsCK8h3pcq0PB5zvJE0OdTrGA70khAYJQy4",
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
