#!/bin/sh
# Install github.com/sasha-s/go-deadlock and use it to test mutexes
go get github.com/sasha-s/go-deadlock/...@master
find . -name "*.go" -exec sed -i '' 's#"sync"#"sync"\
	"github.com/sasha-s/go-deadlock"#' {} +
find . -name "*.go" -exec sed -i '' 's#sync.RWMutex#deadlock.RWMutex#' {} +
find . -name "*.go" -exec sed -i '' 's#sync.Mutex#deadlock.Mutex#' {} +
