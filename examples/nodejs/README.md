# Mercure with nodejs

First thing is to run the hub, the easiest way is to use docker:

```bash
docker run \
    -e PUBLISHER_JWT_KEY=jwtkey -e SUBSCRIBER_JWT_KEY=jwtkey -e ALLOW_ANONYMOUS=1 -e CORS_ALLOWED_ORIGINS="*" \
        -p 80:80 -p 443:443 \
            dunglas/mercure
```

You may change the `PUBLISHER_JWT_KEY` and the `SUBSCRIBER_JWT_KEY`, we will need them after. `CORS_ALLOWED_ORIGINS` is set to `*` and we `ALLOW_ANONYMOUS` subscribers.

## Subscriber

The subscriber is running in a web browser. To use it, run a static server in this directory, for example by using `serve`:

```bash
npm install -g serve
serve
```

Open `localhost:5000` and the browser console.

## Publisher

To use the publisher you will need a valid JWT token, you can create one using an online service for example [this jwt builder](http://jwtbuilder.jamiekurtz.com/).

To publish a new message:

```
PUBLISHER_JWT_TOKEN='your_token_here' node publisher.js
```

You should receive messages in the browser's console.
