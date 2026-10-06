# Mercure

## Build Tags

Always pass the following build tags when building or testing unless explicitly asked not to:
`deprecated_transport,deprecated_topic,deprecated_claim,nobadger,nomysql,nopgx`

For example:

```console
go test -tags "deprecated_transport,deprecated_topic,deprecated_claim,nobadger,nomysql,nopgx" ./...
```
