---
title: "Mercure use cases: AI streaming, real-time UIs, and collaboration"
description: "Practical Mercure use cases including LLM token streaming, AI agent progress, live data, collaborative editing, async jobs, and notifications."
---

# Mercure use cases

Bring your application to life with streaming AI responses, live dashboards, and notifications that arrive as things happen. These guides show how to build them with Mercure and your existing backend.

**[Start with Mercure Cloud](https://mercure.rocks/pricing)** to focus on your application. Need a supported deployment on your own servers? [Mercure Enterprise](../production/high-availability.md) adds clustering, shared transports, and direct access to the maintainers.

## AI streaming

- **[LLM token streaming](llm-token-streaming.md)**: stream tokens from a server-side OpenAI / Anthropic / local-model call to the browser as they arrive, over a shared SSE subscription.
- **[AI agent progress](ai-agent-progress.md)**: push state changes from a long-running agent ("searching the web", "running tool", "summarizing") to the UI in real time.

## Application real-time use cases

- **[Live data and dashboards](live-data.md)**: stock tickers, availability counters, IoT telemetry, observability dashboards.
- **[Collaborative editing](collaborative-editing.md)**: multiple users edit the same document, changes broadcast as they happen.
- **[Async jobs and progress](async-jobs.md)**: kick off a long-running job, push progress to the requester, deliver the result when ready.
- **[Notifications](notifications.md)**: in-app toasts, mention badges, mailbox counters.

## Server-rendered apps with Mercure

- **[Hotwire / Turbo Streams](hotwire.md)**: stream HTML fragments to swap into the page, no JSON layer required.

## API integrations with Mercure

- **[GraphQL subscriptions](graphql.md)**: deliver GraphQL results through Mercure.
- **[Laravel Broadcasting](laravel-broadcasting.md)**: publish Laravel events and receive them with Echo.

## Mercure in production: case studies

These talks and articles describe Mercure deployments:

- [mail.tm reported 8 million notifications per day in 2022.](https://les-tilleuls.coop/en/blog/mail-tm-mercure-rocks-and-api-platform)
- [Mercure deployment examples from a conference talk.](https://speakerdeck.com/dunglas/mercure-real-time-for-php-made-easy?slide=52)
- [Raven Controls: Mercure at Euro 2020 and COP26 (2022 talk).](https://api-platform.com/con/2022/conferences/real-time-and-beyond-with-mercure/)

## Don't see your case?

For help choosing an approach, search or ask in [GitHub Discussions](https://github.com/dunglas/mercure/discussions).
