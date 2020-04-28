#!/bin/bash -x
# Install github.com/sasha-s/go-deadlock and use it to test mutexes

SEP="\n\t"
args=( "-i" )
if [ "$(uname)" = "Darwin" ]; then
    SEP=$'\\\n\\\t'
    args=( "-i" "" )
fi

GO111MODULE=off go get golang.org/x/tools/cmd/goimports
go get github.com/sasha-s/go-deadlock/...@master
find . -name "*.go" -exec sed "${args[@]}" -e "s#\"sync\"#\"sync\"${SEP}deadlock \"github.com/sasha-s/go-deadlock\"#" {} \;
find . -name "*.go" -exec sed "${args[@]}" -e 's#sync.RWMutex#deadlock.RWMutex#' {} {} \;
find . -name "*.go" -exec sed "${args[@]}" -e 's#sync.Mutex#deadlock.Mutex#' {} {} \;
goimports -w hub
