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

IMG=postgres-tools
SUFFIX=v3
DB_VERSION=10.2
TAG="$DB_VERSION-$SUFFIX"

OSM_VER=${OSM_VER:-0.9.1}

DIST="$REPO_ROOT/dist"
mkdir -p "$DIST"

build() {
  pushd "$REPO_ROOT/hack/docker/postgres-tools/$DB_VERSION"

  # Download osm
  wget https://cdn.appscode.com/binaries/osm/${OSM_VER}/osm-alpine-amd64
  chmod +x osm-alpine-amd64
  mv osm-alpine-amd64 osm

  local cmd="docker build --pull -t $DOCKER_REGISTRY/$IMG:$TAG ."
  echo $cmd; $cmd

  rm osm
  popd
}

binary_repo $@
