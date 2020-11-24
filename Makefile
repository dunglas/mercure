.PHONY: build
build:
	goreleaser --snapshot --rm-dist

.PHONY: clean
clean:
	rm -Rf dist caddy/buildenv*

.PHONY: prepare
prepare: clean
	cd caddy && XCADDY_SKIP_CLEANUP=1 xcaddy build --with github.com/dunglas/mercure=../../ --with github.com/dunglas/mercure/caddy=../
	mv caddy/buildenv_* caddy/buildenv
