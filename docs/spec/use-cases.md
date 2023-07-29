
# Case Studies and Use Cases

From building a chat room to expose a fully-featured event store, Mercure covers a broad spectrum of use cases related to realtime and async APIs.

## Case Studies

* [How Raven Controls is using Mercury to power major events such as Cop 21 and Euro 2020](https://api-platform.com/con/2022/conferences/real-time-and-beyond-with-mercure/)
* [Pushing 8 million of Mercure notifications per day to run mail.tm](https://les-tilleuls.coop/en/blog/mail-tm-mercure-rocks-and-api-platform)
* [100,000 simultaneous Mercure users to power iGraal](https://speakerdeck.com/dunglas/mercure-real-time-for-php-made-easy?slide=52)

Example of usage: the Mercure integration in [API Platform](https://api-platform.com/docs/client-generator):

![API Platform screencast](https://raw.githubusercontent.com/api-platform/docs/3.1/create-client/images/create-client-demo.gif)

## Use Cases

Here are some popular use cases:

### Live Availability

* a webapp retrieves the availability status of a product from a REST API and displays it: only one is still available
* 3 minutes later, the last product is bought by another customer
* the webapp's view instantly shows that this product isn't available anymore

### Asynchronous Jobs

* a webapp tells the server to compute a report, this task is costly and will take some time to complete
* the server delegates the computation of the report to an asynchronous worker (using message queue), and closes the connection with the webapp
* the worker sends the report to the webapp when it is computed

### Collaborative Editing

* a webapp allows several users to edit the same document concurrently
* changes made are immediately broadcasted to all connected users

**Mercure gets you covered!**

See also, [the examples](../ecosystem/awesome.md#examples).
