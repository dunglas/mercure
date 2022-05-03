# Awesome Mercure

## Demos

* [Demo hub and debug UI](https://demo.mercure.rocks) ([source code](https://github.com/dunglas/mercure/tree/master/public)): a managed demo hub and the official debugging tools (written in JavaScript)
* [Chat](https://demo-chat.mercure.rocks/) ([source code](https://github.com/dunglas/mercure/tree/master/examples/chat)): a chat, including the list of currently connected users (written in JavaScript and Python)

## Examples

* [JavaScript (publish, subscribe and presence API)](https://github.com/dunglas/mercure/blob/master/public/app.js)
* [JavaScript (subscribe and presence API)](https://github.com/dunglas/mercure/blob/master/examples/chat/static/chat.js)
* [Node.js (publish)](https://github.com/dunglas/mercure/tree/master/examples/publish/node.js)
* [PHP (publish)](https://github.com/dunglas/mercure/tree/master/examples/publish/php.php)
* [Ruby (publish)](https://github.com/dunglas/mercure/tree/master/examples/publish/ruby.rb)
* [Python (subscribe)](https://github.com/dunglas/mercure/tree/master/examples/subscribe/python.py)
* [Python (cookie authorization)](https://github.com/dunglas/mercure/blob/master/examples/chat/chat.py)
* [API Platform (publish and subscribe)](https://github.com/api-platform/demo): a book catalog updated in real-time using Mercure

## Documentation and Code Generation

* The Mercure protocol is natively supported by [the AsyncAPI ecosystem](https://www.asyncapi.com/)

## Hubs and Server Libraries

* [Go Hub and Server library](https://mercure.rocks)
* [Node.js Hub and Server library](https://github.com/Ilshidur/node-mercure)

## Client Libraries

* [PHP (publish)](https://github.com/symfony/mercure)
* [Python (publish and subscribe)](https://github.com/vitorluis/python-mercure)
* [Dart (publish and subscribe)](https://github.com/wallforfry/dart_mercure)
* [Amphp (publish)](https://github.com/eislambey/amp-mercure-publisher)
* [Java (publish)](https://github.com/vitorluis/java-mercure)

## Frameworks and Services Integrations

* [Official Mercure support for the Symfony framework](https://symfony.com/doc/current/mercure.html)
* [Official Mercure support for the API Platform framework](https://api-platform.com/docs/core/mercure/)
* [Using Mercure and Hotwire to stream page changes](hotwire.md)
* [Laravel Mercure Broadcaster](https://github.com/mvanduijker/laravel-mercure-broadcaster)
* [Yii Mercure Behavior](https://github.com/bizley/mercure-behavior)
* [Add a Mercure service in GitHub Actions](github-actions.md)
* [Send a Mercure publish event from GitHub Actions](https://github.com/Ilshidur/action-mercure)

## Useful Related Libraries

* [`EventSource` polyfill for Edge/IE and old browsers](https://github.com/Yaffle/EventSource) 🚨 since version 1.0.26, [this library contains code that could display messages against the war in Ukraine to end users](https://github.com/Yaffle/EventSource/commit/de137927e13d8afac153d2485152ccec48948a7a)
* [`EventSource` polyfill for React Native](https://github.com/jordanbyron/react-native-event-source)
* [`EventSource` implementation for Node](https://github.com/EventSource/eventsource)
* [Server-Sent Events client for Go](https://github.com/donovanhide/eventsource)
* [Server-Sent Events client for Android and Java](https://github.com/heremaps/oksse)
* [Server-Sent Events client for Swift](https://github.com/inaka/EventSource)
* [JavaScript library to parse `Link` headers](https://github.com/thlorenz/parse-link-header)
* [JavaScript library to decrypt JWE using the WebCrypto API](https://github.com/square/js-jose)

## Projects Using Mercure

* [HTTP Broadcast: a scalable and fault resilient HTTP broadcaster](https://github.com/jderusse/http-broadcast)

## Learning Resources

### English

* [Official Push and Real-Time Capabilities for Symfony and API Platform using Mercure (Symfony blog)](https://dunglas.fr/2019/03/official-push-and-real-time-capabilities-for-symfony-and-api-platform-mercure-protocol/)
* [Tech Workshop: Mercure by Kévin Dunglas at SensioLabs (SensioLabs)](https://blog.sensiolabs.com/2019/01/24/tech-workshop-mercure-kevin-dunglas-sensiolabs/)
* [Real-time messages with Mercure using Laravel](http://thedevopsguide.com/real-time-notifications-with-mercure/)
* [Using Mercure on Stackhero](https://www.stackhero.io/en/documentations/mercure-hub/getting-started)

### French

* [Tutoriel vidéo : Notifications instantanées avec Mercure (Grafikart)](https://www.grafikart.fr/tutoriels/symfony-mercure-1151)
* [Mercure, un protocole pour pousser des mises à jour vers des navigateurs et app mobiles en temps réel (Les-Tilleuls.coop)](https://les-tilleuls.coop/fr/blog/article/mercure-un-protocole-pour-pousser-des-mises-a-jour-vers-des-navigateurs-et-app-mobiles-en-temps-reel)

### German

* [Neue Symfony-Komponente: Mercure ermöglicht Echtzeitübertragung](https://entwickler.de/online/php/symfony-mercure-komponente-579885243.html)
