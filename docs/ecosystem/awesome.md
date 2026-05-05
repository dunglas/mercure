---
title: "Awesome Mercure: Libraries, Integrations, Demos, and Tutorials"
description: "Curated list of Mercure libraries, framework integrations, demos, and tutorials for JavaScript, PHP, Python, Go, Java, Dart, Rust, and more."
---

# Awesome Mercure

A curated list of libraries, integrations, and learning resources around Mercure. Add yours via PR.

## Mercure Demos

- [Demo hub and debug UI](https://demo.mercure.rocks) ([source](https://github.com/dunglas/mercure/tree/master/public)) — managed demo hub with the official debug tools.
- [Chat](https://demo-chat.mercure.rocks/) ([source](https://github.com/dunglas/mercure/tree/master/examples/chat)) — chat app with presence (JavaScript + Python).

## Mercure Code Examples

- [JavaScript: publish, subscribe, presence API](https://github.com/dunglas/mercure/blob/master/public/app.js)
- [JavaScript: subscribe + presence](https://github.com/dunglas/mercure/blob/master/examples/chat/static/chat.js)
- [Node.js: publish](https://github.com/dunglas/mercure/tree/master/examples/publish/node.js)
- [PHP: publish](https://github.com/dunglas/mercure/tree/master/examples/publish/php.php)
- [Ruby: publish](https://github.com/dunglas/mercure/tree/master/examples/publish/ruby.rb)
- [Python: subscribe](https://github.com/dunglas/mercure/tree/master/examples/subscribe/python.py)
- [Python: cookie-based authorization](https://github.com/dunglas/mercure/blob/master/examples/chat/chat.py)
- [API Platform: publish + subscribe](https://github.com/api-platform/demo) — book catalog updated in real time.

## Hubs and Server Libraries

- [Mercure.rocks Hub (Go)](https://mercure.rocks) — the reference implementation.
- [Freddie (PHP)](https://github.com/bpolaszek/freddie)
- [Ilshidur/node-mercure (Node.js)](https://github.com/Ilshidur/node-mercure)

## Mercure Client Libraries

- [`@microsoft/fetch-event-source` (JavaScript)](https://github.com/Azure/fetch-event-source) — better SSE client for browsers and Node.
- [`symfony/mercure` (PHP, publisher)](https://github.com/symfony/mercure)
- [`python-mercure` (Python, publish + subscribe)](https://github.com/vitorluis/python-mercure)
- [`dart_mercure` (Dart / Flutter, publish + subscribe)](https://github.com/wallforfry/dart_mercure)
- [`amp-mercure-publisher` (PHP / Amphp)](https://github.com/eislambey/amp-mercure-publisher)
- [`java-mercure` (Java, publisher)](https://github.com/vitorluis/java-mercure)
- [`mercure-rs` (Rust, publisher)](https://github.com/teohhanhui/mercure-rs)

## Mercure Framework Integrations

- [Symfony](https://symfony.com/doc/current/mercure.html) — official component, full publisher support.
- [API Platform](https://api-platform.com/docs/core/mercure/) — full publisher + subscriber + GraphQL subscriptions.
- [Laravel Mercure Broadcaster](https://github.com/mvanduijker/laravel-mercure-broadcaster)
- [Yii Mercure Behavior](https://github.com/bizley/mercure-behavior)
- [CakePHP Mercure plugin](https://github.com/josbeir/cakephp-mercure)
- [Hotwire / Turbo Streams](../use-cases/hotwire.md)
- [GitHub Action: publish on workflow events](https://github.com/Ilshidur/action-mercure)

## Documentation, Tooling, and Code Generation

- [AsyncAPI](https://www.asyncapi.com/) — natively supports the Mercure protocol.
- [URI Template tester](https://uri-template-tester.mercure.rocks/) — for hubs running 0.x-style URI Template subscriptions.

## Useful SSE and JWT Libraries for Mercure

- [`EventSource` polyfill (Edge / IE / old browsers)](https://github.com/Yaffle/EventSource)
- [`EventSource` polyfill for React Native](https://github.com/jordanbyron/react-native-event-source)
- [`EventSource` for Node](https://github.com/EventSource/eventsource)
- [Server-Sent Events client for Go](https://github.com/donovanhide/eventsource)
- [Server-Sent Events client for Android / Java](https://github.com/heremaps/oksse)
- [Server-Sent Events client for Swift](https://github.com/inaka/EventSource)
- [`parse-link-header` (JavaScript)](https://github.com/thlorenz/parse-link-header) — parse `Link: rel="mercure"` headers.
- [`jose` (JavaScript)](https://github.com/panva/jose) — JWT and JWE in the browser and Node.

## Projects Using Mercure

- [HTTP Broadcast: scalable HTTP broadcaster](https://github.com/jderusse/http-broadcast)

## Mercure Learning Resources

### Mercure Resources in English

- 📺 [API updates in real time with Mercure.rocks](https://www.youtube.com/watch?v=odNsxoHSkT4)
- 📺 [Building async public APIs using HTTP/2+ and the Mercure protocol](https://www.youtube.com/watch?v=IUx47Tx0O8E)
- 📺 [Real-time notifications with Symfony and Mercure (basics)](https://www.youtube.com/watch?v=kYNC47V7R_0)
- 📺 [Real-time chat app with Symfony and Mercure](https://www.youtube.com/watch?v=wnr2A4aKnPU)
- [Official push and real-time capabilities for Symfony and API Platform](https://dunglas.fr/2019/03/official-push-and-real-time-capabilities-for-symfony-and-api-platform-mercure-protocol/)
- [Tech workshop: Mercure by Kévin Dunglas](https://blog.sensiolabs.com/2019/01/24/tech-workshop-mercure-kevin-dunglas-sensiolabs/)
- [Real-time messages with Mercure using Laravel](http://thedevopsguide.com/real-time-notifications-with-mercure/)
- [Using Mercure on Stackhero](https://www.stackhero.io/services/Mercure-Hub/documentations)

### Mercure Resources in French

- 📺 [Notifications instantanées avec Mercure (Grafikart)](https://www.grafikart.fr/tutoriels/symfony-mercure-1151)
- 📺 [Live coding: Notifications temps réel avec Mercure](https://www.youtube.com/watch?v=tqqJ1ul2M-E)
- 📺 [Server-Sent Events avec Mercure](https://www.youtube.com/watch?v=Q4LRN2wXuIc)
- 📺 [Mercure: des UIs synchronisées avec les données en BDD](https://www.youtube.com/watch?v=UcBa4AugNTE)
- 📺 [Async avec Messenger, AMQP et Mercure](https://www.youtube.com/watch?v=cHPbcuydJiA)
- [Mercure, un protocole pour pousser des mises à jour en temps réel (Les-Tilleuls.coop)](https://les-tilleuls.coop/blog/mercure-un-protocole-pour-pousser-des-mises-a-jour-vers-des-navigateurs-et-app-mobiles-en-temps-reel)
- [Symfony et Mercure](https://afsy.fr/avent/2019/21-symfony-et-mercure)
- [À la découverte de Mercure](https://blog.eleven-labs.com/fr/a-la-decouverte-de-mercure/)

### Mercure Resources in German

- [Neue Symfony-Komponente: Mercure ermöglicht Echtzeitübertragung](https://entwickler.de/online/php/symfony-mercure-komponente-579885243.html)

## Find More Mercure Projects

[GitHub `mercure` topic](https://github.com/topics/mercure) — community-tagged repositories.

## Add Your Mercure Library or Tutorial

PRs welcome. Keep the structure (one bullet per item, language tag for libraries, language flag for learning resources) and aim for things people will actually find useful.
