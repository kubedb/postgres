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

# ref:
# Prometheus: https://prometheus.io/docs/instrumenting/exporters/
# Github: https://github.com/wrouesnel/postgres_exporter/releases
# Docker: https://hub.docker.com/r/wrouesnel/postgres_exporter/tags

DOCKER_REGISTRY=${DOCKER_REGISTRY:-kubedb}

IMG_REGISTRY=wrouesnel
IMG=postgres_exporter
TAG=v0.4.6

docker pull "$IMG_REGISTRY/$IMG:$TAG"

docker tag "$IMG_REGISTRY/$IMG:$TAG" "$DOCKER_REGISTRY/$IMG:$TAG"
docker push "$DOCKER_REGISTRY/$IMG:$TAG"
