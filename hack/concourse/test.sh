#!/usr/bin/env bash

set -eoux pipefail

ORG_NAME=kubedb
REPO_NAME=postgres
OPERATOR_NAME=pg-operator
APP_LABEL=kubedb #required for `kubectl describe deploy -n kube-system -l app=$APP_LABEL`

export APPSCODE_ENV=dev
export DOCKER_REGISTRY=kubedbci

# get concourse-common
pushd $REPO_NAME
git status # required, otherwise you'll get error `Working tree has modifications.  Cannot add.`. why?
git subtree pull --prefix hack/libbuild https://github.com/appscodelabs/libbuild.git master --squash -m 'concourse'
popd

source $REPO_NAME/hack/libbuild/concourse/init.sh

cp creds/gcs.json /gcs.json
cp creds/.env $GOPATH/src/github.com/$ORG_NAME/$REPO_NAME/hack/config/.env

pushd "$GOPATH"/src/github.com/$ORG_NAME/$REPO_NAME

./hack/builddeps.sh
./hack/docker/$OPERATOR_NAME/make.sh build
./hack/docker/$OPERATOR_NAME/make.sh push
./hack/docker/postgres/9.6.7/make.sh build
./hack/docker/postgres/9.6.7/make.sh push
./hack/docker/postgres/9.6/make.sh
./hack/docker/postgres/10.2/make.sh build
./hack/docker/postgres/10.2/make.sh push

# run tests
./hack/deploy/setup.sh --docker-registry=kubedbci
./hack/make.py test e2e --v=1 --storageclass=$StorageClass --selfhosted-operator=true --ginkgo.flakeAttempts=2
