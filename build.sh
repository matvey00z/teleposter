#!/bin/sh

export GOPATH=$(pwd) # TODO get script location
go get "golang.org/x/net/proxy"
go get "github.com/mattn/go-sqlite3"
go build

