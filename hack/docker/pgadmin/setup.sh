#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

LIB_ROOT=$(dirname "${BASH_SOURCE}")/../../../..
source "$LIB_ROOT/hack/libbuild/common/lib.sh"
source "$LIB_ROOT/hack/libbuild/common/public_image.sh"

IMG=pgadmin
TAG=4-1.1

binary_repo $@
