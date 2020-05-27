
# Use Cases

From building a chat room to expose a fully-featured event store, Mercure covers a broad spectrum of use cases related to realtime and async APIs.

Example of usage: the Mercure integration in [API Platform](https://api-platform.com/docs/client-generator):

![API Platform screencast](https://api-platform.com/d20c0f7f49b5655a3788d9c570c1c80a/client-generator-demo.gif)

Here are some popular use cases:

## Live Availability

* a webapp retrieves the availability status of a product from a REST API and displays it: only one is still available
* 3 minutes later, the last product is bought by another customer
* the webapp's view instantly shows that this product isn't available anymore

## Asynchronous Jobs

* a webapp tells the server to compute a report, this task is costly and will take some time to complete
* the server delegates the computation of the report to an asynchronous worker (using message queue), and closes the connection with the webapp
* the worker sends the report to the webapp when it is computed

## Collaborative Editing

* a webapp allows several users to edit the same document concurrently
* changes made are immediately broadcasted to all connected users

**Mercure gets you covered!**

See also, [the examples](../ecosystem/awesome.md#examples).
