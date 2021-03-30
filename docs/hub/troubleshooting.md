# Troubleshooting

## 401 Unauthorized

* Double-check that the request to the hub includes a `mercureAuthorization` cookie or an `Authorization` HTTP header
* If the cookie isn't set, you may have to explicitly include [the request credentials](https://developer.mozilla.org/en-US/docs/Web/API/WindowOrWorkerGlobalScope/fetch#Parameters) (`new EventSource(url, {withCredentials: true})` and `fetch(url, {credentials: 'include'})`)
* Check the logs written by the hub on `stderr`, they contain the exact reason why the token has been rejected
* Be sure to set a **secret key** (and not a JWT) in `JWT_KEY` (or in `SUBSCRIBER_JWT_KEY` and `PUBLISHER_JWT_KEY`)
* If the secret key contains special characters, be sure to escape them properly, especially if you set the environment variable in a shell, or in a YAML file (Kubernetes...)
* The publisher always needs a valid JWT, even if the `anonymous` directive is present in the `Caddyfile`, this JWT **must** have a property named `publish`. To dispatch private updates, the `publish` property must contain the list of topic selectors this publisher can use ([example](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwie3NjaGVtZX06Ly97K2hvc3R9L2RlbW8vYm9va3Mve2lkfS5qc29ubGQiLCIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey90b3BpY317L3N1YnNjcmliZXJ9Il0sInBheWxvYWQiOnsidXNlciI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdXNlcnMvZHVuZ2xhcyIsInJlbW90ZUFkZHIiOiIxMjcuMC4wLjEifX19.z5YrkHwtkz3O_nOnhC_FP7_bmeISe3eykAkGbAl5K7c))
* The subscriber needs a valid JWT only if the `anonymous` directive isn't present in the `Caddyfile`, or to subscribe to private updates, in this case the JWT **must** have a property named `subscribe` and containing an array of topic selectors ([example](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwie3NjaGVtZX06Ly97K2hvc3R9L2RlbW8vYm9va3Mve2lkfS5qc29ubGQiLCIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey90b3BpY317L3N1YnNjcmliZXJ9Il0sInBheWxvYWQiOnsidXNlciI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdXNlcnMvZHVuZ2xhcyIsInJlbW90ZUFkZHIiOiIxMjcuMC4wLjEifX19.z5YrkHwtkz3O_nOnhC_FP7_bmeISe3eykAkGbAl5K7c))

For both the `publish` property, the array can be empty to publish only public updates. For both `publish` and `subscribe`, you can use `["*"]` to match all topics.

## CORS Issues

If the app connecting to the Mercure hub and the hub itself are not served from the same domain, you must whitelist the domain of the app using the CORS (Cross-Origin Resource Sharing) mechanism. The usual symptoms of a CORS misconfiguration are errors about missing CORS HTTP headers in the console of the browser (`Refused to connect to 'https://hub.example.com/.well-known/mercure?topic=foo' because it violates the following Content Security Policy directive` with Chrome, `Cross-Origin Request Blocked: The Same Origin Policy disallows reading the remote resource at https://hub.example.com/.well-known/mercure?topic=foo. (Reason: CORS header ‘Access-Control-Allow-Origin’ missing)` with Firefox).

To fix these errors, set the list of domains allowed to connect to the hub as value of the `cors_origins` in the `Caddyfile`. Example: `cors_origins https://example.com https://example.net`. Don't forget the `https://` prefix before the domain name!

If you use an authorization mechanism (cookie or `Authorization` header), [you cannot set the value of `cors_origins` to `*`](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS#Credentialed_requests_and_wildcards). You **must** explicitly set the list of allowed origins.
If you don't use an authorization mechanism (anonymous mode), you can set the value of `cors_origins` to `*` to allow all applications to connect to the hub (be sure to understand the security implications of what you are doing).

If you use the Symfony CLI and want to use an external hub, be sure to read [this issue](https://github.com/symfony/cli/issues/424).

## URI Templates and Topics

Try [our URI template tester](https://uri-template-tester.mercure.rocks/) to ensure that the template matches the topic.

## Mac OS Localhost Installation Error

How to process for Mercure to work in Mac OS Catalina:

* In the Finder on your Mac, locate the app that you want to open
* Control-click on the app icon, then choose "Open" from the shortcut menu
* Click "Open"

Then you will have a warning, ignore it and close the Terminal.

Open a new Terminal in the Mercure folder.

Then just start the server:

    ./mercure run

It will work. 🎊
