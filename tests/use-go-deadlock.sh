#!/bin/bash -x
# Install github.com/sasha-s/go-deadlock and use it to test mutexes

SEP="\n\t"
args=( "-i" )
if [[ "$(uname)" = "Darwin" ]]; then
    SEP=$'\\\n\\\t'
    args=( "-i" "" )
fi

go install golang.org/x/tools/cmd/goimports@latest
find . -name "*.go" -exec sed "${args[@]}" -e "s#\"sync\"#\"sync\"${SEP}deadlock \"github.com/sasha-s/go-deadlock\"#" {} \;
find . -name "*.go" -exec sed "${args[@]}" -e 's#sync.RWMutex#deadlock.RWMutex#' {} {} \;
find . -name "*.go" -exec sed "${args[@]}" -e 's#sync.Mutex#deadlock.Mutex#' {} {} \;
goimports -w .
go get github.com/sasha-s/go-deadlock/...@master
cd caddy || exit
go get github.com/sasha-s/go-deadlock/...@master
