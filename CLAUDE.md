# Mercure

## Build Tags

Always pass the following build tags when building or testing unless explicitly asked not to:
`deprecated_server,deprecated_transport,nobadger,nomysql,nopgx`

For example:

```console
go test -tags "deprecated_server,deprecated_transport,nobadger,nomysql,nopgx" ./...
```
