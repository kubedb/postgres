#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

GOPATH=$(go env GOPATH)
REPO_ROOT=$GOPATH/src/github.com/k8sdb/postgres

source "$REPO_ROOT/hack/libbuild/common/lib.sh"
source "$REPO_ROOT/hack/libbuild/common/public_image.sh"

IMG=postgres
TAG=9.5-v3

docker_names=( \
	"db" \
	"util" \
)

DIST=$REPO_ROOT/dist
mkdir -p $DIST
if [ -f "$DIST/.tag" ]; then
	export $(cat $DIST/.tag | xargs)
fi

build() {
        pushd $REPO_ROOT/hack/docker/postgres/9.5
	for name in "${docker_names[@]}"
	do
		cd $name
		docker build -t appscode/$IMG:$TAG-$name .
		cd ..
	done
	popd
}

docker_push() {
	for name in "${docker_names[@]}"
	do
		docker_up $IMG:$TAG-$name
	done
}

docker_release() {
	for name in "${docker_names[@]}"
	do
                docker push appscode/$IMG:$TAG-$name
	done
}

docker_check() {
	for i in "${docker_names[@]}"
	do
		echo "Chcking $IMG ..."
		name=$i-$(date +%s | sha256sum | base64 | head -c 8 ; echo)
		docker run -d -P -it --name=$name appscode/$IMG:$TAG-$i
		docker exec -it $name ps aux
		sleep 5
		docker exec -it $name ps aux
		docker stop $name && docker rm $name
	done
}

binary_repo $@
