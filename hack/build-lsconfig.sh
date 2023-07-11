#!/usr/bin/env bash

# Copyright 2017 The Caoyingjunz Authors.
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

# TODO: 通过变量指定
API_SERVER="https://192.168.16.200:6443"

NAMESPACE="kube-system"
SECRET="pixiu-ls-token"
KUBECONFIG="pixiu-ls-scheduler.conf"

kubectl get secret $SECRET -n $NAMESPACE -o jsonpath='{.data.ca\.crt}' | base64 --decode > ca.crt
TOKEN=$(kubectl get secret $SECRET -n $NAMESPACE -o jsonpath='{.data.token}' | base64 --decode)

kubectl config set-cluster ls-cluster --server=$API_SERVER --certificate-authority=ca.crt --embed-certs=true --kubeconfig=$KUBECONFIG
kubectl config set-credentials csi-ls-node-sa --token=$TOKEN --kubeconfig=$KUBECONFIG
kubectl config set-context ls-context --cluster=ls-cluster --user=csi-ls-node-sa --namespace=$NAMESPACE --kubeconfig=$KUBECONFIG
kubectl config use-context ls-context --kubeconfig=$KUBECONFIG

echo "completed build" $KUBECONFIG
