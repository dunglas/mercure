# Troubleshooting

## 401 Unauthorized

* Check the logs written by the hub on `stderr`, they contain the exact reason why the token has been rejected
* Be sure to set a **secret key** (and not a JWT) in `JWT_KEY` (or in `SUBSCRIBER_JWT_KEY` and `PUBLISHER_JWT_KEY`)
* If the secret key contains special characters, be sure to escape them properly, especially if you set the environment variable in a shell, or in a YAML file (Kubernetes...)
* The publisher always needs a valid JWT, even if `ALLOW_ANONYMOUS` is set to `1`, this JWT **must** have a property named `publish` and containing an array of targets ([example](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOltdfX0.473isprbLWLjXmAaVZj6FIVkCdjn37SQpGjzWws-xa0))
* The subscriber needs a valid JWT only if `ALLOW_ANONYMOUS` is set to `0` (default), or to subscribe to private updates, in this case the JWT **must** have a property named `subscribe` and containing an array of targets ([example](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6W119fQ.s-6MlTvJ6vpsZ7ftmz3dvWpZznRxnxI0KlrZOHVo8Qc))

For both the `publish` and `subscribe` properties, the array can be empty to publish only public updates, or set it to `["*"]` to allow accessing to all targets.

## Browser Issues

If subscribing to the `EventSource` in the browser doesn't work (the browser instantly disconnects from the stream or complains about CORS policy on receiving an event), check that you've set a proper value for `CORS_ALLOWED_ORIGINS` on running Mercure. It's fine to use `CORS_ALLOWED_ORIGINS=*` for your local development.

## URI Template and Topics

Try [our URI template tester](https://uri-template-tester.mercure.rocks/) to ensure that the template matches the topic.

## Mac OS Catalina Localhost Installation Error

How to process for Mercure to work in Mac OS Catalina:

- In the Finder on your Mac, locate the app that you want to open.
- Control-click on the app icon, then choose Open from the shortcut menu.
- Click Open.

Then you will have a warning, ignore it and close the Terminal.

Open a new Terminal in the Mercure folder.

Then just run the command line
```
JWT_KEY='!ChangeMe!' ADDR=':3000' DEMO=1 ALLOW_ANONYMOUS=1 CORS_ALLOWED_ORIGINS=* PUBLISH_ALLOWED_ORIGINS='http://localhost:3000' ./mercure
```

It will work.🎊
