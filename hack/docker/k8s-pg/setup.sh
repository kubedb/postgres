#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

GOPATH=$(go env GOPATH)
SRC=$GOPATH/src
BIN=$GOPATH/bin
ROOT=$GOPATH
REPO_ROOT=$GOPATH/src/github.com/k8sdb/postgres

source "$REPO_ROOT/hack/libbuild/common/k8sdb_image.sh"

APPSCODE_ENV=${APPSCODE_ENV:-dev}
IMG=k8s-pg

DIST=$GOPATH/src/github.com/k8sdb/postgres/dist
mkdir -p $DIST
if [ -f "$DIST/.tag" ]; then
	export $(cat $DIST/.tag | xargs)
fi

clean() {
    pushd $REPO_ROOT/hack/docker/k8s-pg
    rm -f k8s-pg Dockerfile
    popd
}

build_binary() {
    pushd $REPO_ROOT
    ./hack/builddeps.sh
    ./hack/make.py build k8s-pg
    detect_tag $DIST/.tag
    popd
}

build_docker() {
    pushd $REPO_ROOT/hack/docker/k8s-pg
    cp $DIST/k8s-pg/k8s-pg-linux-amd64 k8s-pg
    chmod 755 k8s-pg

    cat >Dockerfile <<EOL
FROM alpine

COPY k8s-pg /k8s-pg

USER nobody:nobody
ENTRYPOINT ["/k8s-pg"]
EOL
    local cmd="docker build -t k8sdb/$IMG:$TAG ."
    echo $cmd; $cmd

    rm k8s-pg Dockerfile
    popd
}

build() {
    build_binary
    build_docker
}

docker_push() {
    if [ "$APPSCODE_ENV" = "prod" ]; then
        echo "Nothing to do in prod env. Are you trying to 'release' binaries to prod?"
        exit 1
    fi
    if [ "$TAG_STRATEGY" = "git_tag" ]; then
        echo "Are you trying to 'release' binaries to prod?"
        exit 1
    fi
    hub_canary
}

docker_release() {
    if [ "$APPSCODE_ENV" != "prod" ]; then
        echo "'release' only works in PROD env."
        exit 1
    fi
    if [ "$TAG_STRATEGY" != "git_tag" ]; then
        echo "'apply_tag' to release binaries and/or docker images."
        exit 1
    fi
    hub_up
}

source_repo $@
