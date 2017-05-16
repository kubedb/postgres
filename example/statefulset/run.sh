#!/usr/bin/env bash

REPO_ROOT=$GOPATH/src/github.com/k8sdb/postgres
example=$REPO_ROOT/example/statefulset
kubectl create -f $example
