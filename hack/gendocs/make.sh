#!/usr/bin/env bash

pushd $GOPATH/src/kubedb.dev/postgres/hack/gendocs
go run main.go
popd
