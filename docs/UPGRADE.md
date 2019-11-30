# Upgrade

## 0.8

* According to the new version of the spec, the URL of the Hub changed moved from `/hub` to `/.well-known/mercure`
* `HISTORY_CLEANUP_FREQUENCY`, `HISTORY_SIZE` and `DB_PATH` environment variables have been replaced by the new `TRANSPORT_URL` environment variable
* Lists in `ACME_HOSTS`, `CORS_ALLOWED_ORIGINS`, `PUBLISH_ALLOWED_ORIGINS` must now be space separated
* The public API of the Go library has been totally revamped
