#!/bin/bash
# Install github.com/sasha-s/go-deadlock and use it to test mutexes

SEP="\n\t"
if [ "$(uname)" = "Darwin" ]; then
    SEP=$'\\\n\\\t'
fi

go get github.com/sasha-s/go-deadlock/...@master
find . -name "*.go" -exec sed -i '' "s#\"sync\"#\"sync\"${SEP}deadlock \"github.com/sasha-s/go-deadlock\"#" {} +
find . -name "*.go" -exec sed -i '' 's#sync.RWMutex#deadlock.RWMutex#' {} +
find . -name "*.go" -exec sed -i '' 's#sync.Mutex#deadlock.Mutex#' {} +
goimports -w hub
