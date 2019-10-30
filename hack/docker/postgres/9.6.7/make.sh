#!/bin/bash

# Copyright The KubeDB Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -xeou pipefail

GOPATH=$(go env GOPATH)
REPO_ROOT=$GOPATH/src/kubedb.dev/postgres

source "$REPO_ROOT/hack/libbuild/common/lib.sh"
source "$REPO_ROOT/hack/libbuild/common/kubedb_image.sh"

DOCKER_REGISTRY=${DOCKER_REGISTRY:-kubedb}

IMG=postgres
SUFFIX=v6
DB_VERSION=9.6.7
TAG="$DB_VERSION-$SUFFIX"

WALG_VER=${WALG_VER:-0.2.13-ac}

DIST="$REPO_ROOT/dist"
mkdir -p "$DIST"

build_binary() {
  make build
}

build_docker() {
  pushd "$REPO_ROOT/hack/docker/postgres/$DB_VERSION"

  # Download wal-g
  wget https://github.com/kubedb/wal-g/releases/download/${WALG_VER}/wal-g-alpine-amd64
  chmod +x wal-g-alpine-amd64
  mv wal-g-alpine-amd64 wal-g

  # Copy pg-operator
  cp "$REPO_ROOT/bin/linux_amd64/pg-operator" pg-operator
  chmod 755 pg-operator

  local cmd="docker build --pull -t $DOCKER_REGISTRY/$IMG:$TAG ."
  echo $cmd; $cmd

  rm wal-g pg-operator
  popd
}

build() {
  build_binary
  build_docker
}

binary_repo $@
