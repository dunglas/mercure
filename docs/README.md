---
title: "Mercure Documentation"
description: "Index for the Mercure protocol and Mercure.rocks Hub documentation: real-time SSE push, JWT authorization, deployment, and AI streaming use cases."
---

# Mercure Documentation

Mercure is a real-time protocol built on HTTP and Server-Sent Events. The reference hub is open-source (AGPL-3.0), production-ready, and used to push billions of messages a month.

This documentation covers the protocol and the Mercure.rocks Hub for the **1.0 release**. If you're upgrading from 0.x, start with the [upgrade guide](UPGRADE.md).

## Get Started with Mercure

- [Introduction](introduction.md): what Mercure is and when to reach for it
- [Quickstart](getting-started/quickstart.md): running hub, first subscription, first update in five minutes
- [Installation](getting-started/installation.md): binary, Docker, Compose, Kubernetes, AUR

## Mercure Core Concepts

- [Topics and matchers](concepts/topics-and-matchers.md): how subscribers say what they want
- [Subscribing](concepts/subscribing.md): the SSE side
- [Publishing](concepts/publishing.md): the POST side
- [Authorization](concepts/authorization.md): JWTs, claims, cookies
- [Reconnection and history](concepts/reconnection-and-history.md): `Last-Event-ID`, replay
- [Active subscriptions](concepts/active-subscriptions.md): presence and the subscription API
- [Encryption](concepts/encryption.md): JWE end-to-end

## Mercure Use Cases

- [Use cases overview](use-cases/README.md)
- [LLM token streaming](use-cases/llm-token-streaming.md)
- [AI agent progress](use-cases/ai-agent-progress.md)
- [Live data and dashboards](use-cases/live-data.md)
- [Collaborative editing](use-cases/collaborative-editing.md)
- [Async jobs and progress](use-cases/async-jobs.md)
- [Notifications](use-cases/notifications.md)
- [Hotwire / Turbo Streams](use-cases/hotwire.md)
- [GraphQL subscriptions](use-cases/graphql.md)

## Mercure Hub Deployment

- [Configuration](deployment/configuration.md): Caddyfile directives and environment variables
- [Docker](deployment/docker.md)
- [Kubernetes](deployment/kubernetes.md)
- [Reverse proxies](deployment/reverse-proxy.md): NGINX and Traefik
- [GitHub Actions](deployment/github-actions.md)

## Running Mercure in Production

- [High availability](production/high-availability.md): scaling beyond one node
- [Rolling updates](production/rolling-updates.md): graceful shutdown for SSE
- [Health checks and monitoring](production/health-monitoring.md)
- [Load testing](production/load-testing.md)
- [Debugging](production/debugging.md)
- [Troubleshooting](production/troubleshooting.md)

## Mercure Protocol Reference

- [Protocol](reference/protocol.md): the IETF specification
- [FAQ](reference/faq.md)
- [License](reference/license.md)
- [Upgrade guide](UPGRADE.md)

## Mercure Ecosystem

- [Awesome Mercure](ecosystem/awesome.md): libraries, integrations, demos
- [Conformance tests](ecosystem/conformance-tests.md)

## Mercure Support and Community

- [GitHub Discussions](https://github.com/dunglas/mercure/discussions) for community questions
- [Stack Overflow `mercure` tag](https://stackoverflow.com/questions/tagged/mercure)
- [`#mercure` on the Symfony Slack](https://symfony.com/slack)
- Cloud and Enterprise support: [contact@mercure.rocks](mailto:contact@mercure.rocks)
