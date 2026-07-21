# Awesome Mercure

## Demos

- [Demo hub and debug UI](https://demo.mercure.rocks) ([source code](https://github.com/dunglas/mercure/tree/master/public)): a managed demo hub and the official debugging tools (written in JavaScript)
- [Chat](https://demo-chat.mercure.rocks/) ([source code](https://github.com/dunglas/mercure/tree/master/examples/chat)): a chat app, including the list of currently connected users (written in JavaScript and Python)

## Examples

- [JavaScript (publish, subscribe, and presence API)](https://github.com/dunglas/mercure/blob/master/public/app.js)
- [JavaScript (subscribe and presence API)](https://github.com/dunglas/mercure/blob/master/examples/chat/static/chat.js)
- [Node.js (publish)](https://github.com/dunglas/mercure/tree/master/examples/publish/node.js)
- [PHP (publish)](https://github.com/dunglas/mercure/tree/master/examples/publish/php.php)
- [Ruby (publish)](https://github.com/dunglas/mercure/tree/master/examples/publish/ruby.rb)
- [Python (subscribe)](https://github.com/dunglas/mercure/tree/master/examples/subscribe/python.py)
- [Python (cookie authorization)](https://github.com/dunglas/mercure/blob/master/examples/chat/chat.py)
- [API Platform (publish and subscribe)](https://github.com/api-platform/demo): a book catalog updated in real time using Mercure

## Documentation and Code Generation

- The Mercure protocol is natively supported by [the AsyncAPI ecosystem](https://www.asyncapi.com/)

## Hubs and Server Libraries

- [Go Hub and Server library](https://mercure.rocks)
- [Node.js Hub and Server library](https://github.com/Ilshidur/node-mercure)

## Client Libraries

- [PHP (publish)](https://github.com/symfony/mercure)
- [Python (publish and subscribe)](https://github.com/vitorluis/python-mercure)
- [Dart (publish and subscribe)](https://github.com/wallforfry/dart_mercure)
- [Amphp (publish)](https://github.com/eislambey/amp-mercure-publisher)
- [Java (publish)](https://github.com/vitorluis/java-mercure)
- [Rust (publish)](https://github.com/teohhanhui/mercure-rs)

## Frameworks and Services Integrations

- [Official Mercure support for the Symfony framework](https://symfony.com/doc/current/mercure.html)
- [Official Mercure support for the API Platform framework](https://api-platform.com/docs/core/mercure/)
- [Using Mercure and Hotwire to stream page changes](hotwire.md)
- [Laravel Mercure Broadcaster](https://github.com/mvanduijker/laravel-mercure-broadcaster)
- [Yii Mercure Behavior](https://github.com/bizley/mercure-behavior)
- [CakePHP Mercure plugin](https://github.com/josbeir/cakephp-mercure)
- [Add a Mercure service in GitHub Actions](github-actions.md)
- [Send a Mercure publish event from GitHub Actions](https://github.com/Ilshidur/action-mercure)

## Useful Related Libraries

- [Microsoft's Fetch Event Source, a better JavaScript API for making Event Source requests](https://github.com/Azure/fetch-event-source)
- [`EventSource` polyfill for Edge/IE and old browsers](https://github.com/Yaffle/EventSource)
- [`EventSource` polyfill for React Native](https://github.com/jordanbyron/react-native-event-source)
- [`EventSource` implementation for Node](https://github.com/EventSource/eventsource)
- [Server-Sent Events client for Go](https://github.com/donovanhide/eventsource)
- [Server-Sent Events client for Android and Java](https://github.com/heremaps/oksse)
- [Server-Sent Events client for Swift](https://github.com/inaka/EventSource)
- [JavaScript library to parse `Link` headers](https://github.com/thlorenz/parse-link-header)
- [JavaScript library to decrypt JWE using the WebCrypto API](https://github.com/square/js-jose)

## Projects Using Mercure

- [HTTP Broadcast: a scalable and fault resilient HTTP broadcaster](https://github.com/jderusse/http-broadcast)

## Learning Resources

### English 🇺🇸

- 📺 [API Updates in Real Time w. Mercure.rocks](https://www.youtube.com/watch?v=odNsxoHSkT4)
- 📺 [Building async public APIs using HTTP/2+ and the Mercure protocol](https://www.youtube.com/watch?v=IUx47Tx0O8E)
- [Official Push and Real-Time Capabilities for Symfony and API Platform using Mercure (Symfony blog)](https://dunglas.fr/2019/03/official-push-and-real-time-capabilities-for-symfony-and-api-platform-mercure-protocol/)
- [Real-time with Mercure and FrankenPHP (Will Vincent)](https://willvincent.com/2025/11/29/real-time-with-mercure-and-frankenphp/)
- [Add Realtime Push Capabilities in Symfony With Mercure (Twilio)](https://www.twilio.com/en-us/blog/developers/community/add-realtime-push-capabilities-symfony-mercure)
- [Tech Workshop: Mercure by Kévin Dunglas at SensioLabs (SensioLabs, archived)](https://web.archive.org/web/20240916221517/https://blog.sensiolabs.com/2019/01/24/tech-workshop-mercure-kevin-dunglas-sensiolabs/)
- [Mercure - install and run](https://mysiar.github.io/dev/2020/04/12/mercure-part1.html)
- [Real-time messages with Mercure using Laravel (archived)](https://web.archive.org/web/20191212051423/https://thedevopsguide.com/real-time-notifications-with-mercure/)

### French 🇫🇷

- 📺 [Notifications instantanées avec Mercure (Grafikart)](https://www.grafikart.fr/tutoriels/symfony-mercure-1151)
- 📺 [Live Coding : Notifications temps réel avec Mercure](https://www.youtube.com/watch?v=tqqJ1ul2M-E)
- 📺 [Explication des Server Sent Events (SSE) avec Mercure](https://www.youtube.com/watch?v=Q4LRN2wXuIc)
- 📺 [Mercure : des UIs toujours synchronisées avec les données en BDD](https://www.youtube.com/watch?v=UcBa4AugNTE)
- 📺 [Mercure, et PHP s'enamoure enfin du temps réel](https://www.youtube.com/watch?v=GugURP88Rgg)
- 📺 [Async avec Messenger, AMQP et Mercure](https://www.youtube.com/watch?v=cHPbcuydJiA)
- [Mercure, un protocole pour pousser des mises à jour vers des navigateurs et app mobiles en temps réel (Les-Tilleuls.coop)](https://les-tilleuls.coop/blog/mercure-un-protocole-pour-pousser-des-mises-a-jour-vers-des-navigateurs-et-app-mobiles-en-temps-reel)
- [Symfony et Mercure](https://afsy.fr/avent/2019/21-symfony-et-mercure)
- [À la découverte de Mercure](https://blog.eleven-labs.com/fr/a-la-decouverte-de-mercure/)

### German 🇩🇪

- [Neue Symfony-Komponente: Mercure ermöglicht Echtzeitübertragung](https://entwickler.de/online/php/symfony-mercure-komponente-579885243.html)
