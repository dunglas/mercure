---
title: "Mercure Use Cases: AI Streaming, Real-Time UIs, and Collaboration"
description: "Practical Mercure use cases including LLM token streaming, AI agent progress, live data, collaborative editing, async jobs, and notifications."
---

# Use Cases

Mercure is a thin protocol; it covers a wide range of "I need to push something to a connected client" problems. The pages below are concrete walkthroughs, each one ships a working example you can run.

## Modern AI Workloads

- **[LLM token streaming](llm-token-streaming.md)**: stream tokens from a server-side OpenAI / Anthropic / local-model call to the browser as they arrive, without a WebSocket gateway in front of your inference server.
- **[AI agent progress](ai-agent-progress.md)**: push state changes from a long-running agent ("searching the web", "running tool", "summarizing") to the UI in real time.

## Application Real-Time Use Cases

- **[Live data and dashboards](live-data.md)**: stock tickers, availability counters, IoT telemetry, observability dashboards.
- **[Collaborative editing](collaborative-editing.md)**: multiple users edit the same document, changes broadcast as they happen.
- **[Async jobs and progress](async-jobs.md)**: kick off a long-running job, push progress to the requester, deliver the result when ready.
- **[Notifications](notifications.md)**: in-app toasts, mention badges, mailbox counters.

## Server-Rendered Apps with Mercure

- **[Hotwire / Turbo Streams](hotwire.md)**: stream HTML fragments to swap into the page, no JSON layer required.

## API Integrations with Mercure

- **[GraphQL subscriptions](graphql.md)**: back GraphQL subscriptions with Mercure instead of WebSockets.

## Mercure in Production: Case Studies

Mercure is used at scale today, a few public examples:

- [mail.tm pushes 8M Mercure notifications a day.](https://les-tilleuls.coop/en/blog/mail-tm-mercure-rocks-and-api-platform)
- [iGraal serves 100k concurrent Mercure users.](https://speakerdeck.com/dunglas/mercure-real-time-for-php-made-easy?slide=52)
- [Raven Controls used Mercure to power Cop 21 and Euro 2020.](https://api-platform.com/con/2022/conferences/real-time-and-beyond-with-mercure/)

## Don't See Your Case?

Mercure is the right answer when "the server has fresh data, push it to clients" is the shape of the problem. If you're unsure, [ask in GitHub Discussions](https://github.com/dunglas/mercure/discussions), most "should I use Mercure for X?" questions have already been answered there.
