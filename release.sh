#!/usr/bin/env bash

# Creates the tags for the library and the Caddy module.

set -o nounset
set -o errexit
trap 'echo "Aborting due to errexit on line $LINENO. Exit code: $?" >&2' ERR
set -o errtrace
set -o pipefail
set -o xtrace

if [ $# -ne 1 ]; then
    echo "Usage: ./release.sh version" >&2
    exit 1;
fi

git checkout main
git pull

cd caddy/
go get "github.com/dunglas/mercure@v$1"
cd -

git commit -S -a -m "chore(caddy): bump Mercure to version $1"

git tag -s -m "Version $1" "v$1"
git tag -s -m "Version $1" "caddy/v$1"
git push --follow-tags
