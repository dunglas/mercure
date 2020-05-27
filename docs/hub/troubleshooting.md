# Troubleshooting

## 401 Unauthorized

* Double-check that the request to the hub includes a `mercureAuthorization` cookie or an `Authorization` HTTP header
* If the cookie isn't set, you may have to explicitly include [the request credentials](https://developer.mozilla.org/en-US/docs/Web/API/WindowOrWorkerGlobalScope/fetch#Parameters) (`new EventSource(url, {withCredentials: true})` and `fetch(url, {credentials: 'include'})`)
* Check the logs written by the hub on `stderr`, they contain the exact reason why the token has been rejected
* Be sure to set a **secret key** (and not a JWT) in `JWT_KEY` (or in `SUBSCRIBER_JWT_KEY` and `PUBLISHER_JWT_KEY`)
* If the secret key contains special characters, be sure to escape them properly, especially if you set the environment variable in a shell, or in a YAML file (Kubernetes...)
* The publisher always needs a valid JWT, even if `ALLOW_ANONYMOUS` is set to `1`, this JWT **must** have a property named `publish`. To dispatch private updates, the `publish` property must contain the list of topic selectors this publisher can use ([example](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwie3NjaGVtZX06Ly97K2hvc3R9L2RlbW8vYm9va3Mve2lkfS5qc29ubGQiLCIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey90b3BpY317L3N1YnNjcmliZXJ9Il0sInBheWxvYWQiOnsidXNlciI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdXNlcnMvZHVuZ2xhcyIsInJlbW90ZUFkZHIiOiIxMjcuMC4wLjEifX19.z5YrkHwtkz3O_nOnhC_FP7_bmeISe3eykAkGbAl5K7c))
* The subscriber needs a valid JWT only if `ALLOW_ANONYMOUS` is set to `0` (default), or to subscribe to private updates, in this case the JWT **must** have a property named `subscribe` and containing an array of topic selectors ([example](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwie3NjaGVtZX06Ly97K2hvc3R9L2RlbW8vYm9va3Mve2lkfS5qc29ubGQiLCIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey90b3BpY317L3N1YnNjcmliZXJ9Il0sInBheWxvYWQiOnsidXNlciI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdXNlcnMvZHVuZ2xhcyIsInJlbW90ZUFkZHIiOiIxMjcuMC4wLjEifX19.z5YrkHwtkz3O_nOnhC_FP7_bmeISe3eykAkGbAl5K7c))

For both the `publish` property, the array can be empty to publish only public updates. For both `publish` and `subscribe`, you can use `["*"]` to match all topics.

## Browser Issues

If subscribing to the `EventSource` in the browser doesn't work (the browser instantly disconnects from the stream or complains about CORS policy on receiving an event), check that you've set a proper value for `CORS_ALLOWED_ORIGINS` on running Mercure. It's fine to use `CORS_ALLOWED_ORIGINS=*` for your local development.

## URI Templates and Topics

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
