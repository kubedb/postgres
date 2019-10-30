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

set -eou pipefail

GOPATH=$(go env GOPATH)
REPO_ROOT=$GOPATH/src/kubedb.dev/postgres

export DB_UPDATE=1
export TOOLS_UPDATE=1
export EXPORTER_UPDATE=1
export OPERATOR_UPDATE=1

show_help() {
  echo "update-docker.sh [options]"
  echo " "
  echo "options:"
  echo "-h, --help                       show brief help"
  echo "    --db-only                    update only database images"
  echo "    --tools-only                 update only database-tools images"
  echo "    --exporter-only              update only database-exporter images"
  echo "    --operator-only              update only operator image"
}

while test $# -gt 0; do
  case "$1" in
    -h | --help)
      show_help
      exit 0
      ;;
    --db-only)
      export DB_UPDATE=1
      export TOOLS_UPDATE=0
      export EXPORTER_UPDATE=0
      export OPERATOR_UPDATE=0
      shift
      ;;
    --tools-only)
      export DB_UPDATE=0
      export TOOLS_UPDATE=1
      export EXPORTER_UPDATE=0
      export OPERATOR_UPDATE=0
      shift
      ;;
    --exporter-only)
      export DB_UPDATE=0
      export TOOLS_UPDATE=0
      export EXPORTER_UPDATE=1
      export OPERATOR_UPDATE=0
      shift
      ;;
    --operator-only)
      export DB_UPDATE=0
      export TOOLS_UPDATE=0
      export EXPORTER_UPDATE=0
      export OPERATOR_UPDATE=1
      shift
      ;;
    *)
      show_help
      exit 1
      ;;
  esac
done

dbversions=(
  9.6.7
  9.6
  10.2
  10.6
  11.1
  11.2
)

exporters=(
  v0.4.6
  v0.4.7
)

echo ""
env | sort | grep -e DOCKER_REGISTRY -e APPSCODE_ENV || true
echo ""

if [ "$DB_UPDATE" -eq 1 ]; then
  cowsay -f tux "Processing database images" || true
  for db in "${dbversions[@]}"; do
    ${REPO_ROOT}/hack/docker/postgres/${db}/make.sh build
    ${REPO_ROOT}/hack/docker/postgres/${db}/make.sh push
  done
fi

if [ "$TOOLS_UPDATE" -eq 1 ]; then
  cowsay -f tux "Processing database-tools images" || true
  for db in "${dbversions[@]}"; do
    ${REPO_ROOT}/hack/docker/postgres-tools/${db}/make.sh build
    ${REPO_ROOT}/hack/docker/postgres-tools/${db}/make.sh push
  done
fi

if [ "$EXPORTER_UPDATE" -eq 1 ]; then
  cowsay -f tux "Processing database-exporter images" || true
  for exporter in "${exporters[@]}"; do
    ${REPO_ROOT}/hack/docker/postgres_exporter/${exporter}/make.sh
  done
fi
