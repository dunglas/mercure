#!/usr/bin/env bash

# Generates a Profile-Guided Optimization profile for the Caddy-embedded
# Mercure binary by driving a short Gatling run against it and capturing a
# CPU profile from Caddy's admin pprof endpoint. The profile is written to
# caddy/mercure/default.pgo (auto-consumed by `go build` for release builds)
# and to default.pgo at the module root (for library consumers to reference
# via -pgo=...).

set -o nounset
set -o errexit
trap 'echo "Aborting due to errexit on line $LINENO. Exit code: $?" >&2' ERR
set -o errtrace
set -o pipefail
set -o xtrace

for cmd in git go yq curl grep java openssl; do
	if ! type "$cmd" >/dev/null; then
		echo "The \"$cmd\" command must be installed." >&2
		exit 1
	fi
done

cd "$(git rev-parse --show-toplevel)"

if [[ "$(uname)" == "Darwin" ]]; then
	ulimit -n 16384
fi

hub_pid=""
gatling_pid=""
sub_api_pid=""
tmp="$(mktemp -d)"
cleanup() {
	for pid in "$sub_api_pid" "$gatling_pid" "$hub_pid"; do
		if [[ -n "$pid" ]]; then
			kill "$pid" 2>/dev/null || true
		fi
	done
	rm -rf "$tmp"
}
trap cleanup EXIT

# Build tags must match the caddy build in .goreleaser.yml so the profile
# reflects the code that actually ships. .golangci.yml is deliberately wider
# (it adds deprecated_server for lint coverage) and is not the right source.
tags="$(yq '.builds[] | select(.id == "caddy") | .tags | join(",")' .goreleaser.yml)"

GOFLAGS='-pgo=off' go build -C caddy/mercure -tags "$tags" -o "$tmp/mercure" .

# Matches the test JWT shipped by the Gatling simulation in gatling/ and the
# keys documented in docs/hub/install.md.
export MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!'
export MERCURE_PUBLISHER_JWT_ALG=HS256
export MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!'
export MERCURE_SUBSCRIBER_JWT_ALG=HS256
# Matches the mix of traffic most production hubs see: anonymous SSE
# subscribers for public browser traffic, and the subscription API enabled
# for operators.
export MERCURE_EXTRA_DIRECTIVES=$'anonymous\n\tsubscriptions'
# Gatling (JSSE/Netty) strips SNI for single-label hostnames like localhost,
# so tell Caddy to serve the localhost cert on empty-SNI handshakes.
export GLOBAL_OPTIONS='default_sni localhost'
# Use the in-memory local transport rather than the bolt default. The bolt
# write path would otherwise dominate the profile, which over-fits PGO to a
# code path users running Enterprise transports (or local://) don't exercise.
export MERCURE_TRANSPORT_URL='local://'

# Generate an HS256 JWT with publish/subscribe = ["*"] signed with the same
# key the hub is configured with. The Gatling default JWT only authorises
# subscribe selectors for /my-private-topic etc., which would reject the
# random topics that PRIVATE_UPDATES=true produces.
b64url() { openssl base64 -A | tr -d '=' | tr '/+' '_-'; }
jwt_header='eyJhbGciOiJIUzI1NiJ9'
jwt_payload="$(printf '%s' '{"mercure":{"publish":["*"],"subscribe":["*"]}}' | b64url)"
jwt_sig="$(printf '%s.%s' "$jwt_header" "$jwt_payload" \
	| openssl dgst -sha256 -hmac '!ChangeThisMercureHubJWTSecretKey!' -binary \
	| b64url)"
jwt="${jwt_header}.${jwt_payload}.${jwt_sig}"

"$tmp/mercure" run --config Caddyfile --adapter caddyfile &
hub_pid=$!

until curl -fsS http://localhost:2019/config/ >/dev/null 2>&1; do
	sleep 0.2
done

(
	cd gatling
	INJECTION_DURATION=60 CONNECTION_DURATION=60 \
		INITIAL_SUBSCRIBERS=500 \
		SUBSCRIBERS_RATE_FROM=10 SUBSCRIBERS_RATE_TO=50 \
		PUBLISHERS_RATE_FROM=10 PUBLISHERS_RATE_TO=50 \
		PRIVATE_UPDATES=true \
		HUB_URL=https://localhost/.well-known/mercure \
		JWT="$jwt" SUBSCRIBER_JWT="$jwt" \
		./mvnw -q gatling:test
) &
gatling_pid=$!

# Exercise the subscription API during the profile window.
(
	while true; do
		curl -fsSk -H "Authorization: Bearer $jwt" \
			https://localhost/.well-known/mercure/subscriptions >/dev/null 2>&1 || true
		sleep 0.5
	done
) &
sub_api_pid=$!

# Let Gatling reach the post-injection peak before sampling. The simulation
# runs INJECTION_DURATION (60 s) + CONNECTION_DURATION (60 s) = 120 s; waiting
# 60 s captures samples from t=60 onward, when subscriber count is at its max
# and the publisher rate has reached the upper end of its ramp.
sleep 60

curl -fsS -o "$tmp/default.pgo" \
	"http://localhost:2019/debug/pprof/profile?seconds=60"

kill "$sub_api_pid" 2>/dev/null || true
sub_api_pid=""
wait "$gatling_pid" || true

install -m 644 "$tmp/default.pgo" caddy/mercure/default.pgo
install -m 644 "$tmp/default.pgo" default.pgo

go tool pprof -top -nodecount=10 default.pgo
