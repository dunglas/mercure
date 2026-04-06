# Troubleshooting

## 401 Unauthorized

- Double-check that the request to the hub includes an authorization cookie (the default name is `mercureAuthorization`) or an `Authorization` HTTP header
- If the cookie isn't set, you may have to explicitly include [the request credentials](https://developer.mozilla.org/en-US/docs/Web/API/WindowOrWorkerGlobalScope/fetch#Parameters) (`new EventSource(url, {withCredentials: true})` and `fetch(url, {credentials: 'include'})`)
- Check the logs written by the hub on `stderr`, they contain the exact reason why the token has been rejected
- Be sure to set a **secret key** (not a JWT) in `JWT_KEY` (or in `SUBSCRIBER_JWT_KEY` and `PUBLISHER_JWT_KEY`)
- If the secret key contains special characters, be sure to escape them properly, especially if you set the environment variable in a shell, or in a YAML file (Kubernetes...)
- The publisher always needs a valid JWT, even if the `anonymous` directive is present in the `Caddyfile`. This JWT **must** have a property named `publish`. To dispatch private updates, the `publish` property must contain the list of topic matcher objects this publisher can use ([example](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlt7Im1hdGNoIjoiKiJ9XSwic3Vic2NyaWJlIjpbeyJtYXRjaCI6Imh0dHBzOi8vZXhhbXBsZS5jb20vbXktcHJpdmF0ZS10b3BpYyJ9LHsibWF0Y2giOiJodHRwczovL2V4YW1wbGUuY29tL2RlbW8vYm9va3MvOmlkLmpzb25sZCIsIm1hdGNoVHlwZSI6IlVSTFBhdHRlcm4ifSx7Im1hdGNoIjoiLy53ZWxsLWtub3duL21lcmN1cmUvc3Vic2NyaXB0aW9ucy86bWF0Y2hUeXBlLzptYXRjaC86c3Vic2NyaWJlciIsIm1hdGNoVHlwZSI6IlVSTFBhdHRlcm4ifV0sInBheWxvYWQiOnsicmVtb3RlQWRkciI6IjEyNy4wLjAuMSIsInVzZXIiOiJodHRwczovL2V4YW1wbGUuY29tL3VzZXJzL2R1bmdsYXMifX19.-I_LuyEjpjZKSfFI-4BstvrLzdCNslsSjHfR5RX0PcM))
- The subscriber needs a valid JWT only if the `anonymous` directive isn't present in the `Caddyfile`, or to subscribe to private updates. In this case, the JWT **must** have a property named `subscribe` containing an array of topic matcher objects ([example](https://jwt.io/#debugger-io?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlt7Im1hdGNoIjoiKiJ9XSwic3Vic2NyaWJlIjpbeyJtYXRjaCI6Imh0dHBzOi8vZXhhbXBsZS5jb20vbXktcHJpdmF0ZS10b3BpYyJ9LHsibWF0Y2giOiJodHRwczovL2V4YW1wbGUuY29tL2RlbW8vYm9va3MvOmlkLmpzb25sZCIsIm1hdGNoVHlwZSI6IlVSTFBhdHRlcm4ifSx7Im1hdGNoIjoiLy53ZWxsLWtub3duL21lcmN1cmUvc3Vic2NyaXB0aW9ucy86bWF0Y2hUeXBlLzptYXRjaC86c3Vic2NyaWJlciIsIm1hdGNoVHlwZSI6IlVSTFBhdHRlcm4ifV0sInBheWxvYWQiOnsicmVtb3RlQWRkciI6IjEyNy4wLjAuMSIsInVzZXIiOiJodHRwczovL2V4YW1wbGUuY29tL3VzZXJzL2R1bmdsYXMifX19.-I_LuyEjpjZKSfFI-4BstvrLzdCNslsSjHfR5RX0PcM))

For the `publish` property, the array can be empty to allow publishing only public updates. For both `publish` and `subscribe`, you can use `[{"match": "*"}]` to match all topics.

## CORS Issues

If the app connecting to the Mercure hub and the hub itself are not served from the same domain, you must whitelist the domain of the app using the CORS (Cross-Origin Resource Sharing) mechanism.

The usual symptoms of a CORS misconfiguration are errors about missing CORS HTTP headers in the browser console:

- Chrome: `Refused to connect to 'https://hub.example.com/.well-known/mercure?match=foo' because it violates the following Content Security Policy directive`
- Firefox: `Cross-Origin Request Blocked: The Same Origin Policy disallows reading the remote resource at https://hub.example.com/.well-known/mercure?match=foo. (Reason: CORS header ‘Access-Control-Allow-Origin’ missing)`

To fix these errors, set the list of domains allowed to connect to the hub as the value of `cors_origins` in the `Caddyfile`. Example: `cors_origins https://example.com https://example.net`. Don't forget the `https://` prefix before the domain name!

If you use an authorization mechanism (cookie or `Authorization` header), [you **cannot** set the value of `cors_origins` to `*`](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS#Credentialed_requests_and_wildcards). You **must** explicitly set the list of allowed origins.

If you don't use an authorization mechanism (anonymous mode), you can set the value of `cors_origins` to `*` to allow all applications to connect to the hub (be sure to understand the security implications of this setting).

## Matchers and Topics

Try [our URI template tester](https://uri-template-tester.mercure.rocks/) to verify `matchURITemplate` expressions. Check `matchURLPattern` expressions directly with the [URL Pattern API](https://urlpattern.spec.whatwg.org/) in your browser console.

## Disconnection With the Inability To Reconnect After Some Time

If the JWT supplied to the Mercure hub contains [an `exp` (expiration time) claim](https://www.rfc-editor.org/rfc/rfc7519#section-4.1.3) (this is the default for tokens generated with most JWT libraries), the hub will automatically disconnect when the expiry date is reached.
After that, it is no longer possible to reconnect with the same JWT, as it has expired. The hub will return an HTTP 401 error.

One solution is to generate a new, valid JWT before reconnecting.

Although not setting the `exp` claim allows an open-ended connection, this solution is not recommended as it reduces the security of your data (someone with a valid JWT will be able to connect indefinitely, at least until the secret key is changed).

## macOS Localhost Installation Error

To execute the Mercure.rocks binary, you must first [release it from quarantine](https://eclecticlight.co/2023/03/13/ventura-has-changed-app-quarantine-with-a-new-xattr/).

### macOS Ventura And Later Versions

Remove the `com.apple.quarantine` attribute:

    xattr -d com.apple.quarantine ./mercure

You can now start the hub as usual:

    ./mercure run

The attribute only needs to be deleted once.

### macOS Catalina

On macOS Catalina and later versions, follow these steps:

- In the Finder on your Mac, locate the app that you want to open
- Control-click on the app icon, then choose "Open" from the shortcut menu
- Click "Open"

Then you will have a warning, ignore it and close the Terminal.

Open a new Terminal in the Mercure folder.

Then just start the server:

    ./mercure run

It will work. 🎊
