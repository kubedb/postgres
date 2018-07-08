#!/bin/bash
set -xeou pipefail

GOPATH=$(go env GOPATH)
REPO_ROOT=$GOPATH/src/github.com/kubedb/postgres

export DB_UPDATE=1
export TOOLS_UPDATE=1
export OPERATOR_UPDATE=1

show_help() {
  echo "update-docker.sh [options]"
  echo " "
  echo "options:"
  echo "-h, --help                       show brief help"
  echo "    --db-only                    update only database images"
  echo "    --tools-only                 update only database-tools images"
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
      export OPERATOR_UPDATE=0
      shift
      ;;
    --tools-only)
      export DB_UPDATE=0
      export TOOLS_UPDATE=1
      export OPERATOR_UPDATE=0
      shift
      ;;
    --operator-only)
      export DB_UPDATE=0
      export TOOLS_UPDATE=0
      export OPERATOR_UPDATE=1
      shift
      ;;
    *)
      show_help
      exit 1
      ;;
  esac
done

if [ "$DB_UPDATE" -eq 1 ]; then
  $REPO_ROOT/hack/docker/postgres/9.6.7/make.sh build
  $REPO_ROOT/hack/docker/postgres/9.6.7/make.sh push
  $REPO_ROOT/hack/docker/postgres/9.6/make.sh

  $REPO_ROOT/hack/docker/postgres/10.2/make.sh build
  $REPO_ROOT/hack/docker/postgres/10.2/make.sh push
fi

if [ "$TOOLS_UPDATE" -eq 1 ]; then
  $REPO_ROOT/hack/docker/postgres-tools/9.6.7/make.sh build
  $REPO_ROOT/hack/docker/postgres-tools/9.6.7/make.sh push
  $REPO_ROOT/hack/docker/postgres-tools/9.6/make.sh

  $REPO_ROOT/hack/docker/postgres-tools/10.2/make.sh build
  $REPO_ROOT/hack/docker/postgres-tools/10.2/make.sh push
fi

if [ "$OPERATOR_UPDATE" -eq 1 ]; then
  $REPO_ROOT/hack/docker/pg-operator/make.sh build
  $REPO_ROOT/hack/docker/pg-operator/make.sh push
fi
