#!/usr/bin/env bash

set -eoux pipefail

REPO_NAME=postgres
OPERATOR_NAME=pg-operator

# get concourse-common
pushd $REPO_NAME
git status
git subtree pull --prefix hack/concourse/common https://github.com/kubedb/concourse-common.git master --squash -m 'concourse'
popd

source $REPO_NAME/hack/concourse/common/init.sh

cp creds/gcs.json /gcs.json
cp creds/.env $GOPATH/src/github.com/kubedb/$REPO_NAME/hack/config/.env

pushd "$GOPATH"/src/github.com/kubedb/$REPO_NAME

./hack/builddeps.sh
export APPSCODE_ENV=dev
export DOCKER_REGISTRY=kubedbci
./hack/docker/$OPERATOR_NAME/make.sh build
./hack/docker/$OPERATOR_NAME/make.sh push
./hack/docker/postgres/9.6.7/make.sh build
./hack/docker/postgres/9.6.7/make.sh push
./hack/docker/postgres/9.6/make.sh
./hack/docker/postgres/10.2/make.sh build
./hack/docker/postgres/10.2/make.sh push

# run tests
source ./hack/deploy/setup.sh --docker-registry=kubedbci
./hack/make.py test e2e --v=1 --storageclass=$StorageClass --selfhosted-operator=true --ginkgo.flakeAttempts=2
