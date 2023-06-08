#!/usr/bin/env bash

# Copyright 2017 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

CONTROLLER_TOOLS_VERSION=v0.8.0

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null)}
GO_BIN=$(pwd)/vendor/bin

test -s "${GO_BIN}"/controller-gen && "${GO_BIN}"/controller-gen --version | grep -q ${CONTROLLER_TOOLS_VERSION} || \
GOBIN=${GO_BIN} go install sigs.k8s.io/controller-tools/cmd/controller-gen@${CONTROLLER_TOOLS_VERSION}

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of

bash "${CODEGEN_PKG}"/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/caoyingjunz/csi-driver-localstorage/pkg/client github.com/caoyingjunz/csi-driver-localstorage/pkg/apis localstorage:v1 \
  --go-header-file "${SCRIPT_ROOT}"/hack/boilerplate.go.txt \
  --output-base ./

# 拷贝文件
cp -r "${SCRIPT_ROOT}"/github.com/caoyingjunz/csi-driver-localstorage/pkg/client "${SCRIPT_ROOT}"/pkg
cp -r "${SCRIPT_ROOT}"/github.com/caoyingjunz/csi-driver-localstorage/pkg/apis "${SCRIPT_ROOT}"/pkg

# 生成 CRDs
echo "Generating localstorage CRDs"
"${GO_BIN}"/controller-gen crd paths=./pkg/apis/... output:crd:dir=./deploy/crds output:stdout
