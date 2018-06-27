#!/bin/bash
set -xeou pipefail

GOPATH=$(go env GOPATH)
REPO_ROOT=$GOPATH/src/github.com/kubedb/postgres

# $REPO_ROOT/hack/docker/postgres/9.6.7/make.sh build
# $REPO_ROOT/hack/docker/postgres/9.6.7/make.sh push

# $REPO_ROOT/hack/docker/postgres/9.6/make.sh

# $REPO_ROOT/hack/docker/postgres/10.2/make.sh build
# $REPO_ROOT/hack/docker/postgres/10.2/make.sh push

$REPO_ROOT/hack/docker/postgres-tools/9.6.7/make.sh build
$REPO_ROOT/hack/docker/postgres-tools/9.6.7/make.sh push

$REPO_ROOT/hack/docker/postgres-tools/9.6/make.sh push

$REPO_ROOT/hack/docker/postgres-tools/10.2/make.sh build
$REPO_ROOT/hack/docker/postgres-tools/10.2/make.sh push


# $REPO_ROOT/hack/docker/pg-operator/make.sh build
# $REPO_ROOT/hack/docker/pg-operator/make.sh push

